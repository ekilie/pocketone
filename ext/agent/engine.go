package agent

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pockethost/pocketone/ext/agent/engine"
	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
)

// engineModel is a model loaded by the built-in engine (pure Go, no cgo).
type engineModel struct {
	m    *engine.Model
	file string
}

func loadEngine(path string, cfg Config) (localModel, error) {
	m, err := engine.Load(path, engine.LoadOptions{Threads: cfg.Threads, PromptCache: cfg.promptCacheBytes()})
	if err != nil {
		return nil, err
	}
	return &engineModel{m: m, file: strings.TrimSuffix(filepath.Base(path), ".gguf")}, nil
}

func (e *engineModel) name() string { return e.file }
func (e *engineModel) close() error { return e.m.Close() }

func (e *engineModel) chatPrompt(msgs []agentv1.Message) string {
	out := make([]engine.Message, len(msgs))
	for i, m := range msgs {
		out[i] = engine.Message{Role: m.Role, Content: m.Content}
	}
	return e.m.ChatPrompt(out)
}

func (e *engineModel) generate(ctx context.Context, prompt string, o genOptions) (genResult, error) {
	res, err := e.m.Generate(ctx, prompt, engine.GenerateOptions{
		MaxTokens: o.MaxTokens, Temperature: o.Temperature, TopP: 0.9, TopK: 40, Stop: o.Stop, Constraint: o.Constraint,
	}, o.OnText)
	g := genResult{Text: res.Text, InputTokens: res.PromptTokens, OutputTokens: res.OutputTokens, CachedTokens: res.CachedTokens,
		StopReason: stopReason(res.StopReason), Prefill: res.Prefill, Decode: res.Decode}
	if ctx.Err() != nil && g.StopReason == "" {
		g.StopReason = "cancelled"
	}
	return g, err
}

// stopReason maps the engine's reasons onto agent/v1's: "stop", "length" or "cancelled".
func stopReason(r string) string {
	switch r {
	case "eos", "stop":
		return "stop"
	case "callback": // the caller's OnText said stop
		return "cancelled"
	case "length":
		return "length"
	case "canceled":
		return "cancelled"
	}
	return r
}
