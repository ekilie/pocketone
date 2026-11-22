package llm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"

	llmc "pocketone.dev/ext/contracts/llm"
	settingsv1 "pocketone.dev/ext/contracts/settings/v1"
	"pocketone.dev/ext/extkit"
)

// Provider types: the three clouds with their own handling, and any OpenAI-compatible API.
const (
	TypeVultr      = "vultr"
	TypeCloudflare = "cloudflare"
	TypeOpenRouter = "openrouter"
	TypeOpenAI     = "openai"
)

// Types lists the provider types, with what Settings → LLM shows for each.
var Types = []TypeInfo{
	{Type: TypeVultr, Title: "Vultr", Endpoint: "https://api.vultrinference.com/v1", Name: "vultr",
		About: "Vultr Serverless Inference: text, images, speech, transcription, music, video and embeddings."},
	{Type: TypeCloudflare, Title: "Cloudflare Workers AI", Name: "cf", Account: true,
		About: "Workers AI on your Cloudflare account: text, images, speech, transcription and embeddings. Needs the account ID and a token with Workers AI permission."},
	{Type: TypeOpenRouter, Title: "OpenRouter", Endpoint: "https://openrouter.ai/api/v1", Name: "or",
		About: "One key for hundreds of models: text, images, speech, transcription, video and embeddings."},
	{Type: TypeOpenAI, Title: "OpenAI-compatible API", Endpoint: "https://api.openai.com/v1", Name: "openai",
		About: "Any API shaped like OpenAI's: OpenAI, Groq, Together, Ollama or llama-server on localhost. The endpoint includes its version path (/v1)."},
}

// TypeInfo describes a provider type in Settings → LLM.
type TypeInfo struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Endpoint string `json:"endpoint,omitempty"` // the default endpoint ("" for Cloudflare, built from the account)
	Name     string `json:"name"`               // a suggested provider name
	Account  bool   `json:"account,omitempty"`  // needs an account id (Cloudflare)
	About    string `json:"about"`
}

func typeInfo(t string) (TypeInfo, bool) {
	for _, ti := range Types {
		if ti.Type == t {
			return ti, true
		}
	}
	return TypeInfo{}, false
}

// Provider is one connection the app's admins added in Settings → LLM.
type Provider struct {
	Name     string `json:"name"` // what calls name before the slash: "vultr" in "vultr/glm-5.2"
	Type     string `json:"type"`
	Endpoint string `json:"endpoint,omitempty"` // "" uses the type's default
	Account  string `json:"account,omitempty"`  // Cloudflare's account id
	Key      string `json:"key,omitempty"`
	// Models are the enabled models, by the provider's own id. Nil enables every model of a kind PocketOne supports.
	Models []string `json:"models,omitempty"`
	// AllModels distinguishes "every model" (true, Models ignored) from an explicit list, even an empty one.
	AllModels bool `json:"allModels,omitempty"`
}

// enabled reports whether the provider's model id is turned on.
func (p Provider) enabled(id string) bool {
	return p.AllModels || slices.Contains(p.Models, id)
}

// endpoint is the provider's base URL: its own, or its type's default.
func (p Provider) endpoint() string {
	if p.Endpoint != "" {
		return strings.TrimRight(p.Endpoint, "/")
	}
	ti, _ := typeInfo(p.Type)
	return ti.Endpoint
}

// Settings are Settings → LLM: the providers, and each kind's default model. Stored in the app's _params.
type Settings struct {
	Providers []Provider        `json:"providers"`
	Defaults  map[string]string `json:"defaults,omitempty"` // kind → "provider/model"
}

func (s Settings) provider(name string) (Provider, bool) {
	for _, p := range s.Providers {
		if p.Name == name {
			return p, true
		}
	}
	return Provider{}, false
}

const settingsParam = "llm"

// LoadSettings returns the app's LLM settings (empty when its admins never saved any).
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
		return Settings{}, fmt.Errorf("llm: settings: %w", err)
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

// settingsSection is Settings → LLM, over settings/v1.
type settingsSection struct {
	r *router
}

func (s settingsSection) Info() settingsv1.Info {
	return settingsv1.Info{Extension: Name, Title: "LLM", Testable: true,
		Description: "The providers $llm calls (Vultr, Cloudflare, OpenRouter, any OpenAI-compatible API), their models, and each kind's default."}
}

// providerView is a provider as Get returns it: the key never comes back, only whether one is set.
type providerView struct {
	Provider
	KeySet bool `json:"keySet"`
	// Catalog is every model the provider lists of a kind PocketOne supports, each with whether it's enabled.
	Catalog []catalogEntry `json:"catalog"`
	Error   string         `json:"error,omitempty"` // listing the models failed
}

type catalogEntry struct {
	llmc.ModelInfo
	Enabled bool `json:"enabled"`
}

// settingsView is the section's values.
type settingsView struct {
	Providers []providerView    `json:"providers"`
	Defaults  map[string]string `json:"defaults"`
	Types     []TypeInfo        `json:"types"`
	Kinds     []string          `json:"kinds"`
}

func (s settingsSection) Get(ctx context.Context) (json.RawMessage, error) {
	cur, err := LoadSettings(s.r.app)
	if err != nil {
		return nil, err
	}
	v := settingsView{Defaults: cur.Defaults, Types: Types, Kinds: llmc.AllKinds, Providers: []providerView{}}
	if v.Defaults == nil {
		v.Defaults = map[string]string{}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for _, p := range cur.Providers {
		pv := providerView{Provider: p, KeySet: p.Key != "", Catalog: []catalogEntry{}}
		pv.Key = ""
		ms, err := s.r.catalog(ctx, p, false)
		if err != nil {
			pv.Error = err.Error()
		}
		for _, m := range ms {
			pv.Catalog = append(pv.Catalog, catalogEntry{ModelInfo: m, Enabled: p.enabled(m.ID)})
		}
		v.Providers = append(v.Providers, pv)
	}
	return json.Marshal(v)
}

func (s settingsSection) Save(ctx context.Context, values json.RawMessage) (json.RawMessage, error) {
	next, err := s.parse(values)
	if err != nil {
		return nil, err
	}
	if err := SaveSettings(s.r.app, next); err != nil {
		return nil, err
	}
	s.r.forget() // the model lists are listed again with the new settings
	return s.Get(ctx)
}

// Test checks each provider by listing its models with its key (the cheapest call every cloud has), then asks the
// text default a one-word question when there is one.
func (s settingsSection) Test(ctx context.Context, values json.RawMessage) error {
	next, err := s.parse(values)
	if err != nil {
		return err
	}
	if len(next.Providers) == 0 {
		return badRequest("add a provider first")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var problems []string
	for _, p := range next.Providers {
		if _, err := s.r.catalog(ctx, p, true); err != nil {
			problems = append(problems, p.Name+": "+err.Error())
		}
	}
	if def := next.Defaults[llmc.KindText]; def != "" && len(problems) == 0 {
		r := &router{app: s.r.app, fixed: &next}
		if _, err := (textService{r}).Complete(ctx, extkit.Internal, textCompleteInput(def)); err != nil {
			problems = append(problems, "text default "+def+": "+err.Error())
		}
	}
	if len(problems) > 0 {
		return badRequest(strings.Join(problems, "; "))
	}
	return nil
}

func badRequest(msg string) error {
	return &extkit.Error{Code: "bad_request", Message: msg, Status: 400}
}

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// parse checks submitted settings. A provider sent without a key keeps its saved key, but only when its type,
// endpoint and account are unchanged: a key is never sent somewhere it wasn't entered for.
func (s settingsSection) parse(values json.RawMessage) (Settings, error) {
	var in Settings
	if err := json.Unmarshal(values, &in); err != nil {
		return Settings{}, badRequest(`expected {"providers": [...], "defaults": {...}}`)
	}
	cur, err := LoadSettings(s.r.app)
	if err != nil {
		return Settings{}, err
	}
	next := Settings{Defaults: map[string]string{}}
	seen := map[string]bool{}
	for _, p := range in.Providers {
		p.Name, p.Type = strings.TrimSpace(p.Name), strings.TrimSpace(p.Type)
		p.Endpoint, p.Account = strings.TrimRight(strings.TrimSpace(p.Endpoint), "/"), strings.TrimSpace(p.Account)
		if !nameRE.MatchString(p.Name) {
			return Settings{}, badRequest(fmt.Sprintf("provider name %q: lowercase letters, digits, '.', '_' and '-', no slash", p.Name))
		}
		if seen[p.Name] {
			return Settings{}, badRequest(fmt.Sprintf("two providers are named %q", p.Name))
		}
		seen[p.Name] = true
		ti, ok := typeInfo(p.Type)
		if !ok {
			return Settings{}, badRequest(fmt.Sprintf("provider %s: type must be vultr, cloudflare, openrouter or openai", p.Name))
		}
		if p.Endpoint == ti.Endpoint {
			p.Endpoint = ""
		}
		if p.Endpoint != "" && !strings.HasPrefix(p.Endpoint, "http://") && !strings.HasPrefix(p.Endpoint, "https://") {
			return Settings{}, badRequest(fmt.Sprintf("provider %s: the endpoint must start with http:// or https://", p.Name))
		}
		if ti.Account && p.Account == "" && p.Endpoint == "" {
			return Settings{}, badRequest(fmt.Sprintf("provider %s: Cloudflare needs the account ID", p.Name))
		}
		if p.Key == "" {
			if old, ok := cur.provider(p.Name); ok && old.Type == p.Type && old.Endpoint == p.Endpoint && old.Account == p.Account {
				p.Key = old.Key
			}
		}
		next.Providers = append(next.Providers, p)
	}
	for kind, ref := range in.Defaults {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if !slices.Contains(llmc.AllKinds, kind) {
			return Settings{}, badRequest(fmt.Sprintf("defaults: no kind %q", kind))
		}
		prov, _, ok := llmc.SplitModel(ref)
		if !ok || !seen[prov] {
			return Settings{}, badRequest(fmt.Sprintf("default %s: %q isn't provider/model with one of the providers above", kind, ref))
		}
		next.Defaults[kind] = ref
	}
	return next, nil
}
