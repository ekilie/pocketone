package extkit

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// AppSet is a set of per-app extensions a host registers on every app it starts, in this order:
//
//	plan, err := set.Prepare(app, layers)       // resolve and register the Go extensions
//	jsvm.Register(app, jsvm.Config{             // load pb_hooks (ext/jsvm)
//		Extensions: append(set.JSExtensions(app), ...),
//		OnInit:     func(rt js.Runtime) { set.BindJS(app, rt); ... },
//	})
//	lines, err := set.Finish(app, plan)         // seal the app's contracts; lines are its startup lines
//
// ext/standard's Set is the one PocketOne ships. It's an interface here so hosts (ext/pocker) take it without
// importing the extensions in it.
type AppSet interface {
	// Prepare resolves which extensions run on app (layers gives each one's settings for this app; nil means none)
	// and registers them.
	Prepare(app core.App, layers func(name string) Config) (*Plan, error)
	// JSExtensions are the jsvm extensions the set needs on app (pb_hooks providing contracts).
	JSExtensions(app core.App) []any
	// BindJS binds the JS APIs of the extensions that run on app ($mcp, $tokens, ...) on rt.
	BindJS(app core.App, rt js.Runtime)
	// Finish seals app's contracts and returns its startup lines.
	Finish(app core.App, plan *Plan) ([]string, error)
	// Extensions lists every extension in the set, in registration order (on or off for a given app).
	Extensions() []Extension
}
