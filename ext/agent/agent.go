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
	Enabled      *bool       `yaml:"enabled" json:"enabled,omitempty" doc:"false turns the extension off on this app ($agent and agent/v1 are gone)."`
	Model        string      `yaml:"model" json:"model" doc:"The model to run: qwen3-0.6b, qwen3-1.7b or qwen3-4b (downloaded once), hf:<org>/<repo>/<file>.gguf, or a path to a .gguf file. Empty: none."`
	Threads      int         `yaml:"threads" json:"threads" doc:"CPU threads for the model (default: half the cores)."`
	Parallel     int         `yaml:"parallel" json:"parallel" doc:"Answers written at once; the rest wait their turn (default 1)."`
	PromptCache  int         `yaml:"prompt_cache_mb" json:"prompt_cache_mb" doc:"Memory in MB for keeping the start of recent prompts, so one that begins the same way (a preamble, a chat so far) isn't read again (default 256, about 1,100 tokens of a Qwen3 model; -1 turns it off)."`
	MaxTokens    int         `yaml:"max_tokens" json:"max_tokens" doc:"Longest answer, in tokens, when a call doesn't say (default 512)."`
	Timeout      string      `yaml:"timeout" json:"timeout" doc:"Longest a call may take, waiting included (default 2m)."`
	CacheDir     string      `yaml:"cache_dir" json:"cache_dir" doc:"Where downloaded models are kept (default: the user cache folder's pocketone/models)."`
	ThreadMemory int         `yaml:"thread_memory_mb" json:"thread_memory_mb" doc:"Memory in MB for conversations' model state between calls ($agent's thread option), least recently used first (default 512; -1 keeps them on disk only)."`
	ThreadCache  float64     `yaml:"thread_cache_gb" json:"thread_cache_gb" doc:"Disk in GB for conversations' model state, in pb_cache/agent/threads (default 2; -1 turns it off)."`
	PerInstance  PerInstance `yaml:"per_instance" json:"per_instance" doc:"On a multisite host: each instance's share of the model."`
}

// PerInstance is each instance's share of a host's model.
type PerInstance struct {
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens" doc:"Longest answer per call, in tokens (0: max_tokens)."`
	Concurrency int     `yaml:"concurrency" json:"concurrency" doc:"Calls an instance may have waiting or running at once (default 2)."`
	PinnedMB    int     `yaml:"pinned_mb" json:"pinned_mb" doc:"Memory in MB an instance's warmed prompts may hold in the prompt cache (default 64)."`
	ThreadCache float64 `yaml:"thread_cache_gb" json:"thread_cache_gb" doc:"Disk in GB for an instance's conversation state, in its own pb_cache (default 2.5, a quarter of a 10 GB instance)."`
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
	if c.PerInstance.PinnedMB == 0 {
		c.PerInstance.PinnedMB = 64
	}
	if c.ThreadCache == 0 {
		c.ThreadCache = 2
	}
	if c.PerInstance.ThreadCache == 0 {
		c.PerInstance.ThreadCache = 2.5
	}
	return c
}

// threadMemoryBytes is thread_memory_mb (default 512), 0 when turned off.
func (c Config) threadMemoryBytes() int64 {
	switch {
	case c.ThreadMemory < 0:
		return 0
	case c.ThreadMemory == 0:
		return 512 << 20
	}
	return int64(c.ThreadMemory) << 20
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
			hostShares.Range(func(k, _ any) bool { k.(*hostShare).releaseInstance(id); return true })
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
	runner *runner                      // the built-in model in use, if any
	warms  map[string]agentv1.WarmInput // what the app warmed, by id: warmed again on the next model
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
		for id := range a.warms {
			a.runner.unwarm(pinOf("", id))
		}
		a.runner.release()
	}
	a.runner = next
	if next != nil {
		for id, in := range a.warms { // the app's warms carry over to the new model
			in.Wait = ""
			go warmOn(context.Background(), next, "", id, in)
		}
	}
	return next, nil
}

// scope is the app's thread folder and budget (the prompt cache scope is "" on a single app).
func (a *appAgent) scope() scopeOpts {
	if a.cfg.ThreadCache < 0 {
		return scopeOpts{}
	}
	return scopeOpts{ThreadDir: threadDir(a.app), ThreadDisk: gb(a.cfg.ThreadCache)}
}

// Warm implements agent/v1: the app's built-in model reads in's text ahead of the calls that start with it.
func (a *appAgent) Warm(ctx context.Context, _ extkit.Auth, in agentv1.WarmInput) (agentv1.WarmResult, error) {
	id := warmID(extkit.AppID(a.app), in)
	a.mu.Lock()
	if a.warms == nil {
		a.warms = map[string]agentv1.WarmInput{}
	}
	a.warms[id] = in
	a.mu.Unlock()
	c, r, err := a.current()
	switch {
	case err != nil:
		return agentv1.WarmResult{ID: id, Message: err.Error()}, nil
	case c.Mode == ModeAPI:
		return agentv1.WarmResult{ID: id, Message: "answered by the app's own API, which caches prompts itself"}, nil
	case c.Mode != ModeBuiltin:
		return agentv1.WarmResult{ID: id, Message: errOff(c).Error()}, nil
	case r == nil:
		return agentv1.WarmResult{ID: id, Message: "the model starts when the app serves; it warms then"}, nil
	}
	return warmOn(ctx, r, "", id, in), nil
}

// Forget implements agent/v1: releases a warm.
func (a *appAgent) Forget(_ context.Context, _ extkit.Auth, in agentv1.ForgetInput) (agentv1.Done, error) {
	a.mu.Lock()
	delete(a.warms, in.ID)
	r := a.runner
	a.mu.Unlock()
	if r != nil {
		r.unwarm(pinOf("", in.ID))
	}
	return agentv1.Done{OK: true}, nil
}

// Count implements agent/v1.
func (a *appAgent) Count(_ context.Context, _ extkit.Auth, in agentv1.CountInput) (agentv1.CountResult, error) {
	_, r, _ := a.current()
	return countOf(r, in), nil
}

// Threads implements agent/v1.
func (a *appAgent) Threads(context.Context, extkit.Auth) (agentv1.ThreadList, error) {
	_, r, _ := a.current()
	return threadsOf(r, a.scope()), nil
}

// ForgetThread implements agent/v1.
func (a *appAgent) ForgetThread(_ context.Context, _ extkit.Auth, in agentv1.ForgetInput) (agentv1.Done, error) {
	_, r, _ := a.current()
	if r != nil && r.phase.Load().(string) == "ready" {
		sc := a.scope()
		r.model.forgetThread("", in.ID, sc.ThreadDir)
	}
	return agentv1.Done{OK: true}, nil
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
		return r.complete(ctx, in, maxTokens, "runner", onText, a.scope())
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
func (*Extension) TypeScript() string { return typeScript }

const typeScript = `/** A turn of a conversation. */
type AgentMessage = { role: "system" | "user" | "assistant"; content: string }

/** What a call sends: a prompt, or a conversation. */
type AgentInput = {
  system?: string
  prompt?: string
  messages?: AgentMessage[]
  /** A JSON schema the answer must match; the parsed answer is then in data. */
  schema?: object | string
  maxTokens?: number
  temperature?: number
  stop?: string[]
  model?: string
  /** Keep this conversation's model state between calls, so the next turn reads only what's new (the built-in model). */
  thread?: string
  /** How long to wait for the model to be ready, as a duration ("30s"). .async waits agent.timeout by default. */
  wait?: string
}

type AgentAnswer = {
  text: string
  data?: any
  model: string
  backend: "app" | "runner" | "host"
  stopReason?: string
  usage: { inputTokens: number; outputTokens: number; cachedTokens?: number }
}

type AgentStatus = { backend: "" | "app" | "runner" | "host"; model?: string; ready: boolean; progress?: number; message?: string }

type AgentWarm = { id: string; warmed: boolean; tokens?: number; message?: string; forget(): void }

/** The app's language model (agent/v1): its own API, a model PocketOne runs, or its host's. */
declare var $agent: {
  /** Answers. Fails at once with "not_ready" while the model loads; complete.async waits for it (opts.wait). */
  complete: ((opts: AgentInput) => AgentAnswer) & { async(opts: AgentInput): Promise<AgentAnswer> }
  /**
   * Like complete, calling onText with each piece of the answer as it's written (forward it with ctx.res.sse() in a
   * page, for instance); false stops it. With opts.wait, waits for a loading model first, calling onWait(status).
   */
  stream(opts: AgentInput, onText?: (text: string) => boolean | void, w?: { onWait?: (status: AgentStatus) => void }): AgentAnswer
  /** Which model answers, and whether it's ready. */
  status(): AgentStatus
  /**
   * Reads the start of later prompts ahead of them (a preamble), so they skip reading it. Returns at once and warms in
   * the background; warm.async resolves once it's warm. The id is the same for the same text.
   */
  warm: ((opts: { system?: string; messages?: AgentMessage[]; prompt?: string }) => AgentWarm) & {
    async(opts: { system?: string; messages?: AgentMessage[]; prompt?: string; wait?: string }): Promise<AgentWarm>
  }
  /** Releases a warm by id. */
  forget(id: string): void
  /** Tokens in a text or a conversation (exact with the built-in model, else about four characters each). */
  count(textOrMessages: string | AgentMessage[] | { text?: string; system?: string; messages?: AgentMessage[] }): number
  /** The conversations whose model state is kept (the thread option). */
  threads: {
    list(): { id: string; tokens: number; bytes: number; where: "memory" | "disk"; usedAt: string }[]
    forget(id: string): void
  }
  /** Starts a stored chat (agent.chat/v1, the agent-chat extension) and returns its id. */
  newChat(opts?: { title?: string; meta?: Record<string, any> }): string
  /** A stored chat by id, with the options every send uses. */
  chat(id: string, opts?: AgentChatOptions): AgentChat
  chats: {
    /** Chats, most recently updated first; meta keeps those whose meta has each key with that value. */
    list(opts?: { meta?: Record<string, any>; limit?: number; offset?: number }): AgentChatInfo[]
    /** Deletes every chat whose meta matches; returns how many. */
    forget(opts: { meta: Record<string, any> }): number
  }
}

type AgentChatOptions = {
  system?: string
  /** Sent after the question (history shows the question alone): text, or a function of the question. */
  context?: string | ((question: string) => string)
  /** Messages sent before the summary takes over (default 12). */
  window?: number
  /** Fold the oldest messages into the summary once this many are past the window (default 6). */
  summarizeEvery?: number
  maxTokens?: number
  temperature?: number
  wait?: string
}

type AgentChatInfo = { id: string; title: string; summary: string; meta: any; messages: number; created: string; updated: string }

type AgentChat = {
  id: string
  /** Asks and saves both turns. With onText, streams the answer to it. */
  send: ((text: string, opts?: { onText?: (text: string) => boolean | void; onWait?: (s: AgentStatus) => void; schema?: object | string; context?: string | ((q: string) => string) }) => AgentAnswer) & {
    async(text: string, opts?: { schema?: object | string; context?: string | ((q: string) => string) }): Promise<AgentAnswer>
  }
  get(): AgentChatInfo
  history(opts?: { limit?: number; before?: string }): { id: string; role: "user" | "assistant"; content: string; created: string }[]
  summary(): string
  title(text?: string): string
  meta(obj?: Record<string, any>): any
  forget(): void
}
`
