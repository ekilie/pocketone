// Package agent gives every PocketOne app a language model to call: agent/v1, and $agent.complete in pb_hooks. Who
// answers, in order:
//
//  1. what the app's admins choose in Settings → LLM: off, a model PocketOne runs, or their own OpenAI-compatible API;
//  2. else a model PocketOne runs in this process, when pocketone.yaml's agent section names one (downloaded once and
//     cached);
//  3. else, on a multisite host whose agent section names a model, the host's model, shared by every instance with a
//     few calls at once each ([HostExtension]). Step 1 still wins: an instance's admins leave the shared model by
//     setting their own API.
package agent

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	agentv1 "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	settingsv1 "github.com/pockethost/pocketone/ext/contracts/settings/v1"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// Name is the extension's key.
const Name = "agent"

// Config is the `agent` section of pocketone.yaml: the model PocketOne runs itself. On a multisite host it's the
// model every instance shares, with PerInstance's limits.
//
//	agent:
//	  model: qwen3-1.7b
//	  threads: 4
type Config struct {
	Enabled     *bool       `yaml:"enabled" json:"enabled,omitempty" doc:"false turns the extension off on this app ($agent and agent/v1 are gone)."`
	Model       string      `yaml:"model" json:"model" doc:"The model to run: qwen3-0.6b, qwen3-1.7b or qwen3-4b (downloaded once), hf:<org>/<repo>/<file>.gguf, or a path to a .gguf file. Empty: none."`
	Threads     int         `yaml:"threads" json:"threads" doc:"CPU threads for the model (default: half the cores)."`
	Parallel    int         `yaml:"parallel" json:"parallel" doc:"Answers written at once; the rest wait their turn (default 1)."`
	PromptCache int         `yaml:"prompt_cache_mb" json:"prompt_cache_mb" doc:"Memory in MB for keeping the start of recent prompts, so one that begins the same way (a preamble, a chat so far) isn't read again (default 256, about 1,100 tokens of a Qwen3 model; -1 turns it off)."`
	MaxTokens   int         `yaml:"max_tokens" json:"max_tokens" doc:"Longest answer, in tokens, when a call doesn't say (default 512)."`
	Timeout     string      `yaml:"timeout" json:"timeout" doc:"Longest a call may take, waiting included (default 2m)."`
	CacheDir    string      `yaml:"cache_dir" json:"cache_dir" doc:"Where downloaded models are kept (default: the user cache folder's pocketone/models)."`
	PerInstance PerInstance `yaml:"per_instance" json:"per_instance" doc:"On a multisite host: each instance's share of the model."`
}

// PerInstance is each instance's share of a host's model.
type PerInstance struct {
	MaxTokens   int `yaml:"max_tokens" json:"max_tokens" doc:"Longest answer per call, in tokens (0: max_tokens)."`
	Concurrency int `yaml:"concurrency" json:"concurrency" doc:"Calls an instance may have waiting or running at once (default 2)."`
}

func (c Config) withDefaults() Config {
	if c.Threads <= 0 {
		c.Threads = max(1, runtime.NumCPU()/2)
	}
	if c.Parallel <= 0 {
		c.Parallel = 1
	}
	if c.MaxTokens <= 0 {
		c.MaxTokens = 512
	}
	if c.CacheDir == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			dir = os.TempDir()
		}
		c.CacheDir = filepath.Join(dir, "pocketone", "models")
	}
	c.CacheDir = expandHome(c.CacheDir)
	if c.PerInstance.Concurrency <= 0 {
		c.PerInstance.Concurrency = 2
	}
	return c
}

// promptCacheBytes is the built-in model's prompt cache budget: prompt_cache_mb (default 256), 0 when turned off.
func (c Config) promptCacheBytes() int64 {
	switch {
	case c.PromptCache < 0:
		return 0
	case c.PromptCache == 0:
		return 256 << 20
	}
	return int64(c.PromptCache) << 20
}

func (c Config) timeout() time.Duration {
	if d, err := time.ParseDuration(c.Timeout); err == nil && d > 0 {
		return d
	}
	return 2 * time.Minute
}

// Section is the `agent` section.
var Section = extkit.NewSection(Name, Config{}, "The language model PocketOne runs itself for agent/v1 ($agent.complete); on a multisite host, the model every instance shares.")

// Extension provides agent/v1 on an app, and its settings section.
type Extension struct{}

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements extkit.Extension.
func (*Extension) Name() string { return Name }

// Contracts implements extkit.Declarer.
func (*Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("agent/v1"), WorksWith: extkit.IDs("settings/v1")}
}

// Register implements extkit.Extension: Settings → LLM always, and agent/v1 unless the app's host provides it for its
// instances (then the host's provider answers, reading this app's settings for off and its own API).
func (*Extension) Register(app core.App, _ extkit.Config) error {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.UIExtensions = append(se.UIExtensions, core.UIExtension{Name: Name, FS: UI()})
		return se.Next()
	})
	if extkit.HostProvides(app, agentv1.Contract.ID()) {
		settingsv1.Add(app, settingsSection{app: app, hosted: true})
		id := extkit.AppID(app)
		instanceApps.Store(id, app)
		app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
			instanceApps.CompareAndDelete(id, app)
			return e.Next()
		})
		return nil
	}
	cfg, _ := Section.Get()
	a := &appAgent{app: app, cfg: cfg.withDefaults()}
	if cfg.Model != "" {
		if _, err := resolveModel(cfg.Model, a.cfg.CacheDir); err != nil {
			return err
		}
	}
	settingsv1.Add(app, settingsSection{app: app, onChange: func() { _, _, _ = a.current() }})
	// models download and load only once the app serves, never for a command (superuser, migrate, ...)
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		a.serving.Store(true)
		if _, _, err := a.current(); err != nil {
			app.Logger().Error("agent: model", "error", err)
		}
		return e.Next()
	})
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		_, _ = a.use("")
		return e.Next()
	})
	agentv1.Contract.Provide(app, a)
	return nil
}

//go:embed ui
var uiFiles embed.FS

// UI is the dashboard's Settings → LLM screen (main.js).
func UI() fs.FS {
	sub, _ := fs.Sub(uiFiles, "ui")
	return sub
}

// instanceApps maps a host's instance id to its running app, for the host's provider to read the instance's settings.
var instanceApps sync.Map // string → core.App

// appAgent is agent/v1 on a single app: what its admins chose in Settings → LLM, else pocketone.yaml's model.
type appAgent struct {
	app     core.App
	cfg     Config
	serving atomic.Bool // models start only once the app serves

	mu     sync.Mutex
	runner *runner // the built-in model in use, if any
}

// current is what answers now, with the built-in model's runner started (once the app serves) when that's the choice.
func (a *appAgent) current() (choice, *runner, error) {
	c := choose(a.app, a.cfg, false)
	if c.Mode != ModeBuiltin {
		_, err := a.use("")
		return c, nil, err
	}
	if !a.serving.Load() {
		return c, nil, nil
	}
	r, err := a.use(c.Model)
	return c, r, err
}

// use switches the app's built-in model to spec ("" for none), releasing the one it used before.
func (a *appAgent) use(spec string) (*runner, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runner != nil && a.runner.cfg.Model == spec {
		return a.runner, nil
	}
	var next *runner
	if spec != "" {
		cfg := a.cfg
		cfg.Model = spec
		r, err := sharedRunner(cfg, a.app.Logger())
		if err != nil {
			return nil, err
		}
		next = r
	}
	if a.runner != nil {
		a.runner.release()
	}
	a.runner = next
	return next, nil
}

func (a *appAgent) Complete(ctx context.Context, _ extkit.Auth, in agentv1.CompleteInput) (agentv1.CompleteResult, error) {
	return a.answer(ctx, in, nil)
}

// Stream implements agent/v1: the answer as it's written.
func (a *appAgent) Stream(_ context.Context, _ extkit.Auth, in agentv1.CompleteInput) (extkit.Stream, error) {
	if err := checkInput(in); err != nil {
		return extkit.Stream{}, err
	}
	return streamOf(func(ctx context.Context, onText func(string) bool) (agentv1.CompleteResult, error) {
		return a.answer(ctx, in, onText)
	}), nil
}

// streamOf is agent/v1's stream of an answer: a chunk for each piece as it's written, then one with done and the
// whole result. The answer runs when the stream does, with the stream's context; it stops when the receiver is gone.
func streamOf(answer func(ctx context.Context, onText func(string) bool) (agentv1.CompleteResult, error)) extkit.Stream {
	return extkit.Stream{Run: func(ctx context.Context, send func(any) error) error {
		res, err := answer(ctx, func(text string) bool { return send(agentv1.Chunk{Text: text}) == nil })
		if err != nil {
			return err
		}
		_ = send(agentv1.Chunk{Done: true, Result: &res}) // fails only when the receiver is gone
		return nil
	}}
}

// answer is Complete and Stream: onText, when set, gets the answer as it's written.
func (a *appAgent) answer(ctx context.Context, in agentv1.CompleteInput, onText func(string) bool) (agentv1.CompleteResult, error) {
	if err := checkInput(in); err != nil {
		return agentv1.CompleteResult{}, err
	}
	c, r, err := a.current()
	if err != nil {
		return agentv1.CompleteResult{}, &extkit.Error{Code: "model_failed", Status: 503, Message: err.Error()}
	}
	switch c.Mode {
	case ModeAPI:
		return c.API.run(ctx, in, onText)
	case ModeBuiltin:
		if r == nil {
			return agentv1.CompleteResult{}, agentv1.ErrNotReady // chosen; the app isn't serving yet
		}
		maxTokens := in.MaxTokens
		if maxTokens <= 0 {
			maxTokens = a.cfg.MaxTokens
		}
		return r.complete(ctx, in, maxTokens, "runner", onText, "")
	}
	return agentv1.CompleteResult{}, errOff(c)
}

func (a *appAgent) Status(context.Context, extkit.Auth) (agentv1.Status, error) {
	c, r, err := a.current()
	switch {
	case err != nil:
		return agentv1.Status{Backend: "runner", Model: c.Model, Message: err.Error()}, nil
	case c.Mode == ModeAPI:
		return agentv1.Status{Backend: "app", Model: c.API.Model, Ready: true}, nil
	case c.Mode == ModeBuiltin && r == nil:
		return agentv1.Status{Backend: "runner", Model: c.Model, Message: "starts when the app serves"}, nil
	case c.Mode == ModeBuiltin:
		return r.status("runner"), nil
	}
	return agentv1.Status{Message: errOff(c).Error()}, nil
}

func checkInput(in agentv1.CompleteInput) error {
	if strings.TrimSpace(in.Prompt) == "" && len(in.Messages) == 0 {
		return &extkit.Error{Code: "bad_request", Message: "give a prompt or messages", Status: 400}
	}
	if len(in.Schema) > 0 && !json.Valid(in.Schema) {
		return &extkit.Error{Code: "bad_request", Message: "schema must be a JSON schema", Status: 400}
	}
	return nil
}

// BindJS exposes agent/v1 to the app's JS hooks as $agent.
func (*Extension) BindJS(app core.App, rt js.Runtime) {
	call := func() (agentv1.Agent, error) {
		a, ok := agentv1.Contract.Of(app)
		if !ok {
			return nil, agentv1.ErrNotConfigured
		}
		return a, nil
	}
	_ = rt.Set("$agent", map[string]any{
		"complete": func(opts map[string]any) (map[string]any, error) {
			in, err := inputFromJS(opts)
			if err != nil {
				return nil, errors.New("$agent.complete: " + err.Error())
			}
			a, err := call()
			if err != nil {
				return nil, err
			}
			res, err := a.Complete(context.Background(), extkit.Internal, in)
			if err != nil {
				return nil, err
			}
			return toJS(res)
		},
		// $agent.stream(opts, onText): like complete, calling onText(piece) as the answer is written; onText returning
		// false stops it. It runs on the calling VM, so onText can write to the response (ctx.res.sse()) as it goes.
		"stream": js.NativeFunc(func(c js.Call) (any, error) {
			opts, _ := c.Arg(0).Export().(map[string]any)
			if opts == nil {
				return nil, errors.New("$agent.stream: the first argument must be an options object")
			}
			in, err := inputFromJS(opts)
			if err != nil {
				return nil, errors.New("$agent.stream: " + err.Error())
			}
			a, err := call()
			if err != nil {
				return nil, err
			}
			s, err := a.Stream(context.Background(), extkit.Internal, in)
			if err != nil {
				return nil, err
			}
			if s.Close != nil {
				defer s.Close()
			}
			fn := c.Arg(1).Func()
			var res *agentv1.CompleteResult
			var cbErr error
			err = s.Run(context.Background(), func(e any) error {
				ch, _ := e.(agentv1.Chunk)
				if ch.Done {
					res = ch.Result
					return nil
				}
				if fn == nil || ch.Text == "" {
					return nil
				}
				v, err := fn.Call(ch.Text)
				if err != nil {
					cbErr = errors.New(js.Message(err))
					return cbErr
				}
				if v == false {
					return errStopped
				}
				return nil
			})
			if cbErr != nil {
				app.Logger().Warn("$agent.stream: onText threw; the answer stopped", "error", cbErr)
			}
			if err != nil {
				return nil, err
			}
			if res == nil {
				return nil, errors.New("$agent.stream: the answer ended without a result")
			}
			return toJS(res)
		}),
		"status": func() (map[string]any, error) {
			a, err := call()
			if err != nil {
				return map[string]any{"backend": "", "ready": false, "message": err.Error()}, nil
			}
			st, err := a.Status(context.Background(), extkit.Internal)
			if err != nil {
				return nil, err
			}
			return toJS(st)
		},
	})
}

// errStopped: $agent.stream's onText returned false.
var errStopped = errors.New("stopped")

// inputFromJS reads $agent.complete's options through JSON, so they take the contract's field names. schema may be
// an object or a string.
func inputFromJS(opts map[string]any) (agentv1.CompleteInput, error) {
	if s, ok := opts["schema"].(string); ok {
		opts["schema"] = json.RawMessage(s)
	}
	b, err := json.Marshal(opts)
	if err != nil {
		return agentv1.CompleteInput{}, err
	}
	var in agentv1.CompleteInput
	if err := json.Unmarshal(b, &in); err != nil {
		return agentv1.CompleteInput{}, err
	}
	return in, nil
}

func toJS(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	return out, json.Unmarshal(b, &out)
}

// TypeScript implements the jsvm types hook: $agent's declarations.
func (*Extension) TypeScript() string {
	return `/** The app's language model (agent/v1): its own API, a model PocketOne runs, or its host's. */
declare var $agent: {
  /** Answers a prompt. Blocks until the answer is written; throws "not_configured", "not_ready" or "busy". */
  complete(opts: {
    system?: string
    prompt?: string
    messages?: { role: "system" | "user" | "assistant"; content: string }[]
    /** A JSON schema the answer must match; the parsed answer is then in data. */
    schema?: object | string
    maxTokens?: number
    temperature?: number
    stop?: string[]
    model?: string
  }): {
    text: string
    data?: any
    model: string
    backend: "app" | "runner" | "host"
    stopReason?: string
    usage: { inputTokens: number; outputTokens: number }
  }
  /**
   * Like complete, calling onText with each piece of the answer as it's written (forward it with ctx.res.sse() in a
   * page, for instance). Returning false from onText stops the answer. Returns the whole answer, like complete.
   */
  stream(opts: Parameters<typeof $agent.complete>[0], onText?: (text: string) => boolean | void): ReturnType<typeof $agent.complete>
  /** Which model answers, and whether it's ready. */
  status(): {
    backend: "" | "app" | "runner" | "host"
    model?: string
    ready: boolean
    progress?: number
    message?: string
  }
}
`
}
