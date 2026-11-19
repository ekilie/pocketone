package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"

	agentv1 "pocketone.dev/ext/contracts/agent/v1"
	settingsv1 "pocketone.dev/ext/contracts/settings/v1"
	"pocketone.dev/ext/extkit"
)

// What answers an app's prompts, as its admins choose it in Settings → LLM.
const (
	ModeOff     = "off"     // nothing: $agent throws not_configured
	ModeBuiltin = "builtin" // a model PocketOne runs in this process (not under a host)
	ModeHost    = "host"    // the model its multisite host runs for every instance (only under a host)
	ModeAPI     = "api"     // the app's own OpenAI-compatible API
)

// Settings are what an app's admins set in Settings → LLM. Stored in the app's _params. A zero Mode means they never
// chose: pocketone.yaml's agent section decides (its model runs built in, or nothing does), and under a host the host's
// model answers.
type Settings struct {
	Mode     string `json:"mode,omitempty"`
	Model    string `json:"model,omitempty"`    // builtin: the model's name
	Endpoint string `json:"endpoint,omitempty"` // api: the base URL with its version, e.g. https://api.openai.com/v1
	Key      string `json:"key,omitempty"`      // api
	APIModel string `json:"apiModel,omitempty"` // api: the model to ask for
}

const settingsParam = "agent"

// LoadSettings returns the app's LLM settings (the zero value when its admins never saved any).
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

// SaveSettings stores the app's LLM settings.
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

// choice is what answers an app's prompts right now, and why.
type choice struct {
	Mode   string
	Model  string      // builtin: the model's name or spec
	API    *apiBackend // api
	Source string      // "settings" (its admins chose), "pocketone.yaml", "host" (the host's default) or "none"
}

// choose resolves an app's settings: what its admins chose wins; otherwise pocketone.yaml's agent.model runs built
// in, or under a host (hosted) the host's model answers.
func choose(app core.App, cfg Config, hosted bool) choice {
	s, _ := LoadSettings(app)
	switch s.Mode {
	case ModeOff:
		return choice{Mode: ModeOff, Source: "settings"}
	case ModeAPI:
		return choice{Mode: ModeAPI, Source: "settings",
			API: &apiBackend{Endpoint: s.Endpoint, Key: s.Key, Model: s.APIModel, Timeout: cfg.timeout()}}
	case ModeBuiltin:
		if !hosted && s.Model != "" {
			return choice{Mode: ModeBuiltin, Model: s.Model, Source: "settings"}
		}
	case ModeHost:
		if hosted {
			return choice{Mode: ModeHost, Source: "settings"}
		}
	}
	if hosted {
		return choice{Mode: ModeHost, Source: "host"}
	}
	if cfg.Model != "" {
		return choice{Mode: ModeBuiltin, Model: cfg.Model, Source: "pocketone.yaml"}
	}
	return choice{Mode: ModeOff, Source: "none"}
}

// errOff: the app's admins turned the LLM off, or nothing is configured.
func errOff(c choice) error {
	if c.Source == "settings" {
		return &extkit.Error{Code: agentv1.ErrNotConfigured.Code, Status: 503, Message: "the LLM is off for this app (Settings → LLM)"}
	}
	return agentv1.ErrNotConfigured
}

// settingsSection is Settings → LLM. hosted: the app is an instance of a host that provides agent/v1, so the choices
// are off, the host's model, or the app's own API; the host decides which model runs built in. onChange runs after a
// save (the app starts the newly chosen model).
type settingsSection struct {
	app      core.App
	hosted   bool
	onChange func()
}

func (s settingsSection) Info() settingsv1.Info {
	return settingsv1.Info{Extension: Name, Title: "LLM", Testable: true,
		Description: "What answers $agent.complete: nothing, a model PocketOne runs (or your host's), or your own OpenAI-compatible API."}
}

// modelOption is a built-in model the admins can pick.
type modelOption struct {
	Name  string `json:"name"`
	About string `json:"about"`
	Size  int64  `json:"size,omitempty"`
}

// settingsView is the section as Get returns it: the saved choice (the key never comes back), then what's in effect.
type settingsView struct {
	Mode     string `json:"mode"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
	Key      string `json:"key"`
	KeySet   bool   `json:"keySet"`
	APIModel string `json:"apiModel"`

	Effective      string        `json:"effective"`      // the mode in effect
	EffectiveModel string        `json:"effectiveModel"` // builtin or host: the model
	Source         string        `json:"source"`         // settings, pocketone.yaml, host or none
	YAMLModel      string        `json:"yamlModel"`      // pocketone.yaml's agent.model (the host's model, under a host)
	Hosted         bool          `json:"hosted"`
	Models         []modelOption `json:"models,omitempty"` // the built-in choices (none under a host)
}

func (s settingsSection) Get(context.Context) (json.RawMessage, error) {
	cur, err := LoadSettings(s.app)
	if err != nil {
		return nil, err
	}
	cfg, _ := Section.Get()
	c := choose(s.app, cfg.withDefaults(), s.hosted)
	v := settingsView{Mode: cur.Mode, Model: cur.Model, Endpoint: cur.Endpoint, KeySet: cur.Key != "", APIModel: cur.APIModel,
		Effective: c.Mode, EffectiveModel: c.Model, Source: c.Source, YAMLModel: cfg.Model, Hosted: s.hosted}
	if c.Mode == ModeHost {
		v.EffectiveModel = cfg.Model
	}
	if !s.hosted {
		for _, m := range knownModels {
			v.Models = append(v.Models, modelOption{Name: m.Name, About: m.About, Size: m.Size})
		}
		if cfg.Model != "" {
			if _, known := findKnown(cfg.Model); !known {
				v.Models = append(v.Models, modelOption{Name: cfg.Model, About: "from pocketone.yaml"})
			}
		}
	}
	return json.Marshal(v)
}

func (s settingsSection) Save(_ context.Context, values json.RawMessage) (json.RawMessage, error) {
	next, err := s.parse(values)
	if err != nil {
		return nil, err
	}
	if err := SaveSettings(s.app, next); err != nil {
		return nil, err
	}
	if s.onChange != nil {
		s.onChange()
	}
	return s.Get(context.Background())
}

// Test checks the API settings with a one-token prompt (the other modes have nothing to test).
func (s settingsSection) Test(ctx context.Context, values json.RawMessage) error {
	next, err := s.parse(values)
	if err != nil {
		return err
	}
	if next.Mode != ModeAPI {
		return nil
	}
	api := &apiBackend{Endpoint: next.Endpoint, Key: next.Key, Model: next.APIModel, Timeout: 30 * time.Second}
	_, err = api.run(ctx, agentv1.CompleteInput{Prompt: "Say OK.", MaxTokens: 5}, nil)
	return err
}

func badRequest(msg string) error {
	return &extkit.Error{Code: "bad_request", Message: msg, Status: 400}
}

// parse checks submitted settings. Mode "" goes back to pocketone.yaml (or the host). An empty key keeps the saved one,
// but only for the same endpoint: a key is never sent to an endpoint it wasn't entered for.
func (s settingsSection) parse(values json.RawMessage) (Settings, error) {
	var in settingsView
	if err := json.Unmarshal(values, &in); err != nil {
		return Settings{}, badRequest(`expected {"mode": "off" | "builtin" | "api", ...}`)
	}
	next := Settings{Mode: strings.TrimSpace(in.Mode)}
	switch next.Mode {
	case "", ModeOff:
	case ModeHost:
		if !s.hosted {
			return Settings{}, badRequest("there's no host model here: choose off, builtin or api")
		}
	case ModeBuiltin:
		if s.hosted {
			return Settings{}, badRequest("your host decides which model runs built in: choose host, off or api")
		}
		cfg, _ := Section.Get()
		next.Model = strings.TrimSpace(in.Model)
		if _, known := findKnown(next.Model); !known && (next.Model == "" || next.Model != cfg.Model) {
			return Settings{}, badRequest(fmt.Sprintf("model: choose one of %s", knownNames()))
		}
	case ModeAPI:
		next.Endpoint, next.Key, next.APIModel = strings.TrimSpace(in.Endpoint), in.Key, strings.TrimSpace(in.APIModel)
		if !strings.HasPrefix(next.Endpoint, "http://") && !strings.HasPrefix(next.Endpoint, "https://") {
			return Settings{}, badRequest("endpoint must start with http:// or https://")
		}
		if next.Key == "" {
			cur, err := LoadSettings(s.app)
			if err != nil {
				return Settings{}, err
			}
			if cur.Endpoint == next.Endpoint {
				next.Key = cur.Key
			}
		}
	default:
		return Settings{}, badRequest(fmt.Sprintf("mode %q: choose off, builtin, host or api", next.Mode))
	}
	return next, nil
}
