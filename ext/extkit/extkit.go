// Package extkit is PocketOne's extension contract: one extension value that works, unchanged, on every app, however
// it's hosted: a standalone app, or an instance on a multisite host, whichever runtime runs it. An isolate runtime
// (runtime/v1; ext/pocker is one) registers the same value on many apps in one process, so an extension keeps
// per-app state per app ([AppID]).
//
// An extension implements Name and Register, and optionally [JSBinder]:
//
//	type Extension struct{ cfg Config }
//
//	func New(cfg Config) *Extension { return &Extension{cfg: cfg} }
//	func (p *Extension) Name() string { return "hello" }
//	func (p *Extension) Register(app core.App, cfg extkit.Config) error {
//		c := p.cfg
//		if err := cfg.Decode(&c); err != nil { // settings for this app, over the defaults given to New
//			return err
//		}
//		app.OnServe().BindFunc(...) // hooks, routes, collections, dashboard screens
//		return nil
//	}
//
// Usually it's in a set ([AppSet], ext/standard's), which registers it on each app with that app's settings. Without a
// set, register it with [MustRegister] (and [BindJS] from jsvm's OnInit).
package extkit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// Extension is a PocketOne extension.
type Extension interface {
	// Name is the extension's key: its pocketone.yaml section, an instance's `extensions.<name>` block on a multisite
	// host, the prefix of its collections (`_<name>_`) and its name in logs.
	Name() string

	// Register installs the extension on app before it bootstraps: hooks, routes, collections, dashboard screens.
	// cfg holds the settings for this app (empty on a plain single app). It runs on every start of the app.
	Register(app core.App, cfg Config) error
}

// JSBinder is an extension that exposes its Go API to the app's JS hooks (e.g. as a $name global). It's called for each
// JS runtime of an app the extension is registered on. What it exposes must be safe for a hostile tenant to call on
// its own app: on a multisite host, the app's hooks may be a tenant's code.
type JSBinder interface {
	BindJS(app core.App, rt js.Runtime)
}

// Closer is an extension that holds process-wide resources shared by all its apps (a pool, a store, a connection) and
// releases them once every app has stopped. A single app closes it from OnTerminate; on a multisite host the runtime
// closes it once the host has stopped every app (multisite.Host.Closers). The conformance test calls it
// after stopping its apps, so an extension that keeps goroutines across apps implements it.
type Closer interface {
	Close(ctx context.Context) error
}

// Config is an extension's settings for one app, as layers applied in order (on a Pocker host, two: its
// isolate_config section, then the instance's `extensions.<name>`). Each layer is a map from YAML or JSON. The extension's own defaults, the ones given to its
// constructor (and its pocketone.yaml section, see [NewSection]), sit under all of them.
type Config []map[string]any

// Decode applies every layer, in order, onto out (a pointer to a struct with json tags, or a map): keys a layer
// sets replace what earlier layers or out had, keys it doesn't set are kept. Use pointer fields to tell "not set"
// from a zero value.
func (c Config) Decode(out any) error {
	for _, layer := range c {
		if len(layer) == 0 {
			continue
		}
		b, err := json.Marshal(layer)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, out); err != nil {
			return err
		}
	}
	return nil
}

// Enabled reports the `enabled` setting: the last layer that sets it wins, and an extension is on unless a layer says
// `enabled: false`. Hosts check it before calling Register, so extensions don't have to.
func (c Config) Enabled() bool {
	enabled := true
	for _, layer := range c {
		if v, ok := layer["enabled"]; ok {
			if b, isBool := v.(bool); isBool {
				enabled = b
			}
		}
	}
	return enabled
}

// Layer converts a settings value (a map from YAML, or a typed struct a host's instance source set) into a Config
// layer. nil gives nil.
func Layer(v any) (map[string]any, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		return x, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("extension settings must be a map: %w", err)
	}
	return m, nil
}

// MustRegister registers extensions on a single app, with no settings layers (each extension uses its defaults). It
// panics on error, like PocketBase's own MustRegister helpers.
func MustRegister(app core.App, extensions ...Extension) {
	for _, p := range extensions {
		if err := p.Register(app, nil); err != nil {
			panic(fmt.Errorf("extension %s: %w", p.Name(), err))
		}
	}
}

// BindJS calls BindJS on every extension that implements [JSBinder]. Call it from jsvm's OnInit on a single app.
func BindJS(app core.App, rt js.Runtime, extensions ...Extension) {
	for _, p := range extensions {
		if b, ok := p.(JSBinder); ok {
			b.BindJS(app, rt)
		}
	}
}

const appIDKey = "extkit.appId"

// SetAppID records the host's identifier for app (the instance id). A runtime that runs several apps in one process
// calls it before Register.
func SetAppID(app core.App, id string) { app.Store().Set(appIDKey, id) }

// AppID is the host's identifier for app, set with [SetAppID]: the instance id when the app is an isolate (it shares the process).
// It's "" when the app has the process to itself (a standalone app, or an instance its runtime runs in its own process). Use it for whatever must differ per app in shared state: a bucket prefix, a sender
// address, a rate-limit key.
func AppID(app core.App) string {
	id, _ := app.Store().Get(appIDKey).(string)
	return id
}

// Duration is a time.Duration for settings structs passed to [Config.Decode]: it reads "72h" or "90s" (what YAML
// config files hold) as well as a number of nanoseconds.
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		*d = Duration(v)
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("duration must be a string like \"72h\" or a number of nanoseconds: %w", err)
	}
	*d = Duration(n)
	return nil
}
