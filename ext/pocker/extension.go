package pocker

import (
	"context"

	"pocketone.dev/ext/multisite"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"pocketone.dev/ext/extkit/js"
)

// Extension extends a Pocker server (see [Server.Extensions]). An extension implements Name and any of the optional
// interfaces below; the server checks each one with a type assertion.
type Extension interface {
	// Name identifies the extension: its key under an instance's `extensions:` block, and in logs.
	Name() string
}

// AppIniter sets up each instance's app right after it is created, before its JS hooks load and before it
// bootstraps (opens its databases): the place to bind OnBootstrap, OnServe and OnTerminate handlers (e.g. the pitr
// extension). It runs on every start of the app, including restarts. An error fails that start.
type AppIniter interface {
	InitApp(inst Instance, app core.App) error
}

// JSIniter adds globals to each JS runtime of an instance (hooks, routes, migrations), after the sandbox has set
// the runtime up, e.g. an extension exposing its Go API as $name. What it exposes is a sandbox decision: only expose
// what is safe for a hostile tenant to call on its own app.
type JSIniter interface {
	InitJS(inst Instance, app core.App, rt js.Runtime)
}

func chainJSIniters(in []JSIniter) func(Instance, core.App, js.Runtime) {
	if len(in) == 0 {
		return nil
	}
	return func(inst Instance, app core.App, rt js.Runtime) {
		for _, i := range in {
			i.InitJS(inst, app, rt)
		}
	}
}

func chainIniters(in []AppIniter) func(Instance, core.App) error {
	if len(in) == 0 {
		return nil
	}
	return func(inst Instance, app core.App) error {
		for _, i := range in {
			if err := i.InitApp(inst, app); err != nil {
				return err
			}
		}
		return nil
	}
}

// Starter runs background work once the Manager exists, until ctx is done.
type Starter interface {
	Start(ctx context.Context, m *Manager)
}

// DecodeExtensionConfig is [multisite.DecodeExtensionConfig].
func DecodeExtensionConfig[T any](v any, out *T) error {
	return multisite.DecodeExtensionConfig(v, out)
}
