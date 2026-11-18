// Package agent gives every PocketOne app a language model to call: agent/v1, and $agent.complete in pb_hooks. Who
// answers, in order:
//
//  1. the app's own OpenAI-compatible API, when its admins set one in Settings → Agent;
//  2. else a model PocketOne runs in this process, when pocketone.yaml's agent section names one (downloaded once and
//     cached);
//  3. else, on a multisite host whose agent section names a model, the host's model, shared by every instance with a
//     daily allowance each ([HostExtension]). Step 1 still wins: an instance's admins leave the shared model by
//     setting their own API.
package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"

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
	MaxTokens   int         `yaml:"max_tokens" json:"max_tokens" doc:"Longest answer, in tokens, when a call doesn't say (default 512)."`
	Timeout     string      `yaml:"timeout" json:"timeout" doc:"Longest a call may take, waiting included (default 2m)."`
	CacheDir    string      `yaml:"cache_dir" json:"cache_dir" doc:"Where downloaded models are kept (default: the user cache folder's pocketone/models)."`
	PerInstance PerInstance `yaml:"per_instance" json:"per_instance" doc:"On a multisite host: each instance's share of the model."`
}

// PerInstance is each instance's share of a host's model.
type PerInstance struct {
	TokensPerDay int `yaml:"tokens_per_day" json:"tokens_per_day" doc:"Answer tokens an instance may use a day (0: no limit)."`
	MaxTokens    int `yaml:"max_tokens" json:"max_tokens" doc:"Longest answer per call, in tokens (0: max_tokens)."`
	Concurrency  int `yaml:"concurrency" json:"concurrency" doc:"Calls an instance may have waiting or running at once (default 2)."`
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

func (c Config) timeout() time.Duration {
	if d, err := time.ParseDuration(c.Timeout); err == nil && d > 0 {
		return d
	}
	return 2 * time.Minute
}

// Section is the `agent` section.
var Section = extkit.NewSection(Name, Config{}, "The language model PocketOne runs itself for agent/v1 ($agent.complete); on a multisite host, the model every instance shares.")

// Settings are what an app's admins set in Settings → Agent: their own OpenAI-compatible API. Stored in the app's
// _params.
type Settings struct {
	Endpoint string `json:"endpoint"` // the API's base URL with its version, e.g. https://api.openai.com/v1
	Key      string `json:"key,omitempty"`
	Model    string `json:"model"`
}

const settingsParam = "agent"

// LoadSettings returns the app's agent settings (the zero value when its admins never saved any).
func LoadSettings(app core.App) (Settings, error) {
	var raw string
	err := app.DB().NewQuery("SELECT value FROM {{_params}} WHERE id = {:id}").Bind(dbx.Params{"id": settingsParam}).Row(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Settings{}, fmt.Errorf("agent: settings: %w", err)
	}
	return s, nil
}

// SaveSettings stores the app's agent settings.
func SaveSettings(app core.App, s Settings) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	now := types.NowDateTime().String()
	_, err = app.DB().NewQuery(`INSERT INTO {{_params}} (id, value, created, updated) VALUES ({:id}, {:value}, {:now}, {:now})
		ON CONFLICT(id) DO UPDATE SET value = excluded.value, updated = excluded.updated`).
		Bind(dbx.Params{"id": settingsParam, "value": string(b), "now": now}).Execute()
	return err
}

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

// Register implements extkit.Extension: the settings section always (so an instance can leave its host's shared
// model), and agent/v1 unless the app's host provides it for its instances.
func (*Extension) Register(app core.App, _ extkit.Config) error {
	settingsv1.Add(app, settingsSection{app})
	if extkit.HostProvides(app, agentv1.Contract.ID()) {
		// the host answers; it reads this app's settings to honor its own API (see HostExtension)
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
		// start downloading and loading only when the app serves, not for every command (superuser, migrate, ...)
		app.OnServe().BindFunc(func(e *core.ServeEvent) error {
			r, err := sharedRunner(cfg, app.Logger())
			if err != nil {
				return err
			}
			a.runner.Store(r)
			return e.Next()
		})
	}
	agentv1.Contract.Provide(app, a)
	return nil
}

// instanceApps maps a host's instance id to its running app, for the host's provider to read the instance's settings.
var instanceApps sync.Map // string → core.App

// ejected returns the API an app's admins set, if any.
func ejected(app core.App) (*apiBackend, bool) {
	s, err := LoadSettings(app)
	if err != nil || strings.TrimSpace(s.Endpoint) == "" {
		return nil, false
	}
	return &apiBackend{Endpoint: s.Endpoint, Key: s.Key, Model: s.Model}, true
}

// appAgent is agent/v1 on a single app: its own API, else this process's runner.
type appAgent struct {
	app    core.App
	cfg    Config
	runner atomic.Pointer[runner] // set once the app serves
}

func (a *appAgent) Complete(ctx context.Context, _ extkit.Auth, in agentv1.CompleteInput) (agentv1.CompleteResult, error) {
	if err := checkInput(in); err != nil {
		return agentv1.CompleteResult{}, err
	}
	if api, ok := ejected(a.app); ok {
		api.Timeout = a.cfg.timeout()
		return api.complete(ctx, in)
	}
	r := a.runner.Load()
	if r == nil {
		if a.cfg.Model != "" {
			return agentv1.CompleteResult{}, agentv1.ErrNotReady // configured, the app isn't serving yet
		}
		return agentv1.CompleteResult{}, agentv1.ErrNotConfigured
	}
	maxTokens := in.MaxTokens
	if maxTokens <= 0 {
		maxTokens = a.cfg.MaxTokens
	}
	return r.complete(ctx, in, maxTokens, "runner")
}

func (a *appAgent) Status(context.Context, extkit.Auth) (agentv1.Status, error) {
	if api, ok := ejected(a.app); ok {
		return agentv1.Status{Backend: "app", Model: api.Model, Ready: true}, nil
	}
	r := a.runner.Load()
	if r == nil {
		if a.cfg.Model != "" {
			return agentv1.Status{Backend: "runner", Model: a.cfg.Model, Message: "starts when the app serves"}, nil
		}
		return agentv1.Status{Message: agentv1.ErrNotConfigured.Message}, nil
	}
	return r.status("runner"), nil
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

// settingsSection is Settings → Agent: the app's own API.
type settingsSection struct{ app core.App }

func (s settingsSection) Info() settingsv1.Info {
	return settingsv1.Info{Extension: Name, Title: "Agent", Testable: true,
		Description: "Your own OpenAI-compatible API for $agent.complete (OpenAI, OpenRouter, Groq, Ollama...). Leave the endpoint empty to use the model PocketOne or your host runs."}
}

type settingsView struct {
	Endpoint string `json:"endpoint"`
	Key      string `json:"key"`
	KeySet   bool   `json:"keySet"`
	Model    string `json:"model"`
}

func (s settingsSection) Get(context.Context) (json.RawMessage, error) {
	cur, err := LoadSettings(s.app)
	if err != nil {
		return nil, err
	}
	return json.Marshal(settingsView{Endpoint: cur.Endpoint, KeySet: cur.Key != "", Model: cur.Model})
}

func (s settingsSection) Save(_ context.Context, values json.RawMessage) (json.RawMessage, error) {
	next, err := s.parse(values)
	if err != nil {
		return nil, err
	}
	if err := SaveSettings(s.app, next); err != nil {
		return nil, err
	}
	return s.Get(context.Background())
}

func (s settingsSection) Test(ctx context.Context, values json.RawMessage) error {
	next, err := s.parse(values)
	if err != nil {
		return err
	}
	if next.Endpoint == "" {
		return &extkit.Error{Code: "bad_request", Message: "set an endpoint to test", Status: 400}
	}
	api := &apiBackend{Endpoint: next.Endpoint, Key: next.Key, Model: next.Model, Timeout: 30 * time.Second}
	_, err = api.complete(ctx, agentv1.CompleteInput{Prompt: "Say OK.", MaxTokens: 5})
	return err
}

// parse reads submitted settings; an empty key keeps the saved one.
func (s settingsSection) parse(values json.RawMessage) (Settings, error) {
	var in settingsView
	if err := json.Unmarshal(values, &in); err != nil {
		return Settings{}, &extkit.Error{Code: "bad_request", Message: `expected {"endpoint": "https://api.openai.com/v1", "key": "...", "model": "gpt-4o-mini"}`, Status: 400}
	}
	next := Settings{Endpoint: strings.TrimSpace(in.Endpoint), Key: in.Key, Model: strings.TrimSpace(in.Model)}
	if next.Endpoint != "" && !strings.HasPrefix(next.Endpoint, "http://") && !strings.HasPrefix(next.Endpoint, "https://") {
		return Settings{}, &extkit.Error{Code: "bad_request", Message: "endpoint must start with http:// or https://", Status: 400}
	}
	if next.Key == "" {
		cur, err := LoadSettings(s.app)
		if err != nil {
			return Settings{}, err
		}
		next.Key = cur.Key
	}
	return next, nil
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
  /** Answers a prompt. Blocks until the answer is written; throws "not_configured", "not_ready" or "quota". */
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
  /** Which model answers, whether it's ready, and the app's quota on a shared model. */
  status(): {
    backend: "" | "app" | "runner" | "host"
    model?: string
    ready: boolean
    progress?: number
    message?: string
    quota?: { tokensPerDay: number; usedToday: number; maxTokens: number }
  }
}
`
}
