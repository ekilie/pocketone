package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// localModel is a model loaded in this process (the engine package, through engine.go).
type localModel interface {
	chatPrompt(msgs []agentv1.Message) string
	generate(ctx context.Context, prompt string, o genOptions) (genResult, error)
	name() string
	close() error
}

type genOptions struct {
	MaxTokens   int
	Temperature float64
	Stop        []string
}

type genResult struct {
	Text                      string
	InputTokens, OutputTokens int
	StopReason                string
	Prefill, Decode           time.Duration
}

// loadModel opens a GGUF file with the built-in engine. A variable so tests can stand in a fake.
var loadModel = func(path string, threads int) (localModel, error) { return loadEngine(path, threads) }

// runner is one model this process runs, shared by every app that uses it (a single app, or every instance of a
// host). It downloads and loads the model in the background; until then it answers ErrNotReady.
type runner struct {
	cfg Config
	src modelSource
	log *slog.Logger

	progress atomic.Value // float64, while downloading
	phase    atomic.Value // string: "downloading", "loading", "ready", "failed"
	ready    chan struct{}
	err      error
	model    localModel

	slots chan struct{} // generations at once
}

var (
	runnersMu sync.Mutex
	runners   = map[string]*runner{}
)

// sharedRunner returns the process's runner for cfg's model, starting its download and load the first time.
func sharedRunner(cfg Config, log *slog.Logger) (*runner, error) {
	cfg = cfg.withDefaults()
	src, err := resolveModel(cfg.Model, cfg.CacheDir)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s|%d", src.Path, cfg.Threads)
	runnersMu.Lock()
	defer runnersMu.Unlock()
	if r := runners[key]; r != nil {
		return r, nil
	}
	if log == nil {
		log = slog.Default()
	}
	r := &runner{cfg: cfg, src: src, log: log, ready: make(chan struct{}), slots: make(chan struct{}, cfg.Parallel)}
	r.phase.Store("loading")
	r.progress.Store(0.0)
	runners[key] = r
	go r.start()
	return r, nil
}

func (r *runner) start() {
	defer close(r.ready)
	if _, err := os.Stat(r.src.Path); err != nil && r.src.URL != "" {
		r.phase.Store("downloading")
		r.log.Info("agent: downloading model", "model", r.src.Name, "to", r.src.Path, "bytes", r.src.Size)
		t := time.Now()
		if err := fetch(r.src, nil, &r.progress); err != nil {
			r.fail(err)
			return
		}
		r.log.Info("agent: downloaded model", "model", r.src.Name, "took", time.Since(t).Round(time.Second))
	}
	r.phase.Store("loading")
	t := time.Now()
	m, err := loadModel(r.src.Path, r.cfg.Threads)
	if err != nil {
		r.fail(fmt.Errorf("loading %s: %w", r.src.Name, err))
		return
	}
	r.model = m
	r.phase.Store("ready")
	r.log.Info("agent: model ready", "model", r.src.Name, "file", filepath.Base(r.src.Path), "threads", r.cfg.Threads, "took", time.Since(t).Round(time.Millisecond))
}

func (r *runner) fail(err error) {
	r.err = err
	r.phase.Store("failed")
	r.log.Error("agent: model failed", "model", r.src.Name, "error", err)
}

// status is the runner's part of agent/v1's Status.
func (r *runner) status(backend string) agentv1.Status {
	s := agentv1.Status{Backend: backend, Model: r.src.Name}
	switch r.phase.Load().(string) {
	case "ready":
		s.Ready = true
	case "downloading":
		s.Progress, _ = r.progress.Load().(float64)
		s.Message = fmt.Sprintf("downloading %s (%.0f%%)", r.src.Name, s.Progress*100)
	case "failed":
		s.Message = r.err.Error()
	default:
		s.Message = "loading " + r.src.Name
	}
	return s
}

// complete answers in with the local model, after waiting for a free slot. maxTokens caps the answer (the caller's
// limit, after any host cap).
func (r *runner) complete(ctx context.Context, in agentv1.CompleteInput, maxTokens int, backend string) (agentv1.CompleteResult, error) {
	// phase is stored after model (and err), so reading it first makes them visible here
	switch r.phase.Load().(string) {
	case "ready":
	case "failed":
		return agentv1.CompleteResult{}, &extkit.Error{Code: "model_failed", Status: 503, Message: r.err.Error()}
	default:
		st := r.status(backend)
		return agentv1.CompleteResult{}, &extkit.Error{Code: agentv1.ErrNotReady.Code, Status: 503, Message: st.Message}
	}
	ctx, cancel := context.WithTimeout(ctx, r.cfg.timeout())
	defer cancel()
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	case <-ctx.Done():
		return agentv1.CompleteResult{}, &extkit.Error{Code: "busy", Status: 503, Message: "the model is busy; try again shortly"}
	}
	msgs := []agentv1.Message{}
	if sys := systemPrompt(in); sys != "" {
		msgs = append(msgs, agentv1.Message{Role: "system", Content: sys})
	}
	msgs = append(msgs, in.Messages...)
	if in.Prompt != "" {
		msgs = append(msgs, agentv1.Message{Role: "user", Content: in.Prompt})
	}
	temp := 0.7
	if in.Temperature != nil {
		temp = *in.Temperature
	}
	if len(in.Schema) > 0 && in.Temperature == nil {
		temp = 0 // structured answers: the most likely one
	}
	g, err := r.model.generate(ctx, r.model.chatPrompt(msgs), genOptions{MaxTokens: maxTokens, Temperature: temp, Stop: in.Stop})
	if err != nil && g.OutputTokens == 0 {
		return agentv1.CompleteResult{}, err
	}
	out := agentv1.CompleteResult{Text: g.Text, Model: r.src.Name, Backend: backend, StopReason: g.StopReason,
		Usage: agentv1.Usage{InputTokens: g.InputTokens, OutputTokens: g.OutputTokens}}
	if len(in.Schema) > 0 {
		data, jerr := extractJSON(out.Text)
		if jerr != nil {
			return out, &extkit.Error{Code: "bad_answer", Message: "the model's answer isn't JSON: " + jerr.Error(), Status: 502}
		}
		out.Data = data
	}
	return out, nil
}
