package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/agent/engine"
	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// The primitives under agent/v1's newer operations: waiting for the model, warming prompts, counting tokens and
// threads. They're shared by the single-app provider (appAgent) and the host's (hostAgent).

// waitReady waits up to wait (a duration; "" or 0: not at all) for the model to finish downloading and loading.
func (r *runner) waitReady(ctx context.Context, wait string) error {
	d, _ := time.ParseDuration(wait)
	if d <= 0 {
		return nil
	}
	select {
	case <-r.ready:
		return nil
	default:
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-r.ready:
		return nil
	case <-t.C:
		return nil // complete says not_ready with the current progress
	case <-ctx.Done():
		return ctx.Err()
	}
}

// warmJob is one warm: the text, and when it's done what the engine kept.
type warmJob struct {
	scope, prompt string
	done          chan struct{}
	res           engine.WarmResult
	err           error
}

func (j *warmJob) finished() bool {
	select {
	case <-j.done:
		return true
	default:
		return false
	}
}

// warm starts warming prompt for scope under pin (or returns the job already started for that pin). It waits for the
// model, then for calls ahead of it: a warm never makes a call wait.
func (r *runner) warm(scope, pin, prompt string) *warmJob {
	r.warmMu.Lock()
	defer r.warmMu.Unlock()
	if j := r.warms[pin]; j != nil && j.prompt == prompt {
		return j
	}
	j := &warmJob{scope: scope, prompt: prompt, done: make(chan struct{})}
	r.warms[pin] = j
	go func() {
		defer close(j.done)
		<-r.ready
		if r.phase.Load().(string) != "ready" {
			j.err = r.err
			return
		}
		for r.waiting.Load() > 0 { // calls first
			time.Sleep(20 * time.Millisecond)
		}
		r.slots <- struct{}{}
		defer func() { <-r.slots }()
		ctx, cancel := context.WithTimeout(context.Background(), r.cfg.timeout())
		defer cancel()
		j.res, j.err = r.model.warm(ctx, prompt, scope, pin)
	}()
	return j
}

// unwarm releases pin's warm: the cache may evict its text like any other.
func (r *runner) unwarm(pin string) {
	r.warmMu.Lock()
	j := r.warms[pin]
	delete(r.warms, pin)
	r.warmMu.Unlock()
	if j == nil {
		return
	}
	go func() { // after it finishes, if it's still running
		<-j.done
		if r.model != nil {
			r.model.unpin(pin)
		}
	}()
}

// joinWarm waits for a running warm of scope whose text prompt starts with, so the call reuses it instead of reading
// the same text alongside it.
func (r *runner) joinWarm(ctx context.Context, scope, prompt string) {
	r.warmMu.Lock()
	var wait []*warmJob
	for _, j := range r.warms {
		if j.scope == scope && !j.finished() && strings.HasPrefix(prompt, j.prompt) {
			wait = append(wait, j)
		}
	}
	r.warmMu.Unlock()
	for _, j := range wait {
		select {
		case <-j.done:
		case <-ctx.Done():
			return
		}
	}
}

// pinnedBytes is what scope's finished warms hold in memory.
func (r *runner) pinnedBytes(scope string) int64 {
	r.warmMu.Lock()
	defer r.warmMu.Unlock()
	var n int64
	for _, j := range r.warms {
		if j.scope == scope && j.finished() && j.err == nil {
			n += j.res.Bytes
		}
	}
	return n
}

// warmID is a warm's id: the app and the text, so warming the same text again finds the same warm (also after the
// model changes) and two apps never share one.
func warmID(app string, in agentv1.WarmInput) string {
	b, _ := json.Marshal(struct {
		S string            `json:"s"`
		M []agentv1.Message `json:"m"`
		P string            `json:"p"`
	}{in.System, in.Messages, in.Prompt})
	h := sha256.Sum256(append([]byte(app+"\x00"), b...))
	return "w_" + hex.EncodeToString(h[:10])
}

// pinOf is the engine's pin for a warm: scoped, so two apps' identical ids can't release each other's.
func pinOf(scope, id string) string { return scope + "\x00" + id }

func warmMessages(in agentv1.WarmInput) []agentv1.Message {
	var msgs []agentv1.Message
	if in.System != "" {
		msgs = append(msgs, agentv1.Message{Role: "system", Content: in.System})
	}
	msgs = append(msgs, in.Messages...)
	if in.Prompt != "" {
		msgs = append(msgs, agentv1.Message{Role: "user", Content: in.Prompt})
	}
	return msgs
}

// warmOn warms in on r for scope and waits up to in.Wait for it.
func warmOn(ctx context.Context, r *runner, scope, id string, in agentv1.WarmInput) agentv1.WarmResult {
	out := agentv1.WarmResult{ID: id}
	msgs := warmMessages(in)
	if len(msgs) == 0 {
		out.Message = "nothing to warm: give system, messages or prompt"
		return out
	}
	if r.phase.Load().(string) != "ready" {
		// the prompt's text needs the model's template: start it when the model is ready
		go func() {
			<-r.ready
			if r.phase.Load().(string) == "ready" {
				r.warm(scope, pinOf(scope, id), r.model.warmPrompt(msgs))
			}
		}()
		if err := r.waitReady(ctx, in.Wait); err != nil || r.phase.Load().(string) != "ready" {
			out.Message = r.status("").Message + "; it warms once the model is ready"
			return out
		}
	}
	j := r.warm(scope, pinOf(scope, id), r.model.warmPrompt(msgs))
	if d, _ := time.ParseDuration(in.Wait); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-j.done:
		case <-t.C:
		case <-ctx.Done():
		}
	}
	return warmResult(out, j)
}

func warmResult(out agentv1.WarmResult, j *warmJob) agentv1.WarmResult {
	if !j.finished() {
		out.Message = "warming"
		return out
	}
	switch {
	case j.err != nil:
		out.Message = j.err.Error()
	case j.res.Kept == 0:
		out.Tokens = j.res.Tokens
		out.Message = "not kept: the prompt cache is off or full of other warms, or the text is too short to cache"
	default:
		out.Warmed, out.Tokens = true, j.res.Kept
	}
	return out
}

// count is the token count of in: exact with the built-in model when it's ready, else about four characters a token.
func countOf(r *runner, in agentv1.CountInput) agentv1.CountResult {
	msgs := warmMessages(agentv1.WarmInput{System: in.System, Messages: in.Messages})
	if r != nil && r.phase.Load().(string) == "ready" {
		n := 0
		if in.Text != "" {
			n += r.model.count(in.Text)
		}
		if len(msgs) > 0 {
			n += r.model.count(r.model.chatPrompt(msgs))
		}
		return agentv1.CountResult{Tokens: n, Exact: true}
	}
	chars := len(in.Text)
	for _, m := range msgs {
		chars += len(m.Content) + 8 // the role and the turn's markers
	}
	return agentv1.CountResult{Tokens: (chars + 3) / 4}
}

func threadsOf(r *runner, sc scopeOpts) agentv1.ThreadList {
	out := agentv1.ThreadList{Threads: []agentv1.ThreadInfo{}}
	if r == nil || r.phase.Load().(string) != "ready" {
		return out
	}
	for _, t := range r.model.threads(sc.Scope, sc.ThreadDir) {
		out.Threads = append(out.Threads, agentv1.ThreadInfo{ID: t.ID, Tokens: t.Tokens, Bytes: t.Bytes, Where: t.Where, UsedAt: t.UsedAt})
	}
	return out
}

// threadDir is app's folder for thread state: pb_cache/agent/threads (disposable, never backed up).
func threadDir(app core.App) string {
	dir, err := extkit.CacheDir(app, Name)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "threads")
}

// gb is a size in GB as bytes; negative turns it off (0 bytes).
func gb(v float64) int64 {
	if v < 0 {
		return 0
	}
	return int64(v * (1 << 30))
}
