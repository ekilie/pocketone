package main

import (
	"fmt"
	"log/slog"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	pbjsvm "ganigeorgiev.com/pocketbase/pocketbase/plugins/jsvm"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
	"github.com/pockethost/pocketone/ext/jsvm"
	"github.com/pockethost/pocketone/ext/standard"
)

// The JS engine switch (`app: jsvm:` / --jsvm):
//
//   - quickjs (default): ext/jsvm, one QuickJS VM per app, with every PocketOne JS feature ($mcp, $tokens, $bulkmail,
//     pages, JS contracts).
//   - goja: PocketBase's own plugins/jsvm, for pure PocketBase compatibility. The PocketOne JS features are off and
//     the startup lines say so.
//
// A build without cgo has no QuickJS: it defaults to goja.
const (
	engineQuickJS = "quickjs"
	engineGoja    = "goja"
)

func defaultJSEngine() string {
	if jsvm.Available {
		return engineQuickJS
	}
	return engineGoja
}

// gojaOff is what an app gives up in goja mode.
const gojaOff = "pages, $mcp, $tokens, $bulkmail, JS contracts, real globals and async handlers"

func registerSet(app core.App, set *standard.Set, engine string, cfg jsvm.Config) error {
	switch engine {
	case engineQuickJS:
		if !jsvm.Available {
			return fmt.Errorf("--jsvm quickjs: %w", jsvm.ErrNoCgo)
		}
		return set.Register(app, cfg)
	case engineGoja:
		err := set.RegisterWith(app, func(app core.App) error {
			return pbjsvm.Register(app, pbjsvm.Config{
				HooksDir:      cfg.HooksDir,
				HooksWatch:    cfg.HooksWatch,
				HooksPoolSize: cfg.HooksPoolSize,
				MigrationsDir: cfg.MigrationsDir,
			})
		})
		if err != nil {
			return err
		}
		app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
			Func: func(e *core.ServeEvent) error {
				msg := "jsvm: goja (PocketBase's own engine, for compatibility); off: " + gojaOff
				if !jsvm.Available {
					msg += " (this build has no cgo, so no quickjs)"
				}
				slog.Warn(msg)
				return e.Next()
			},
		})
		return nil
	default:
		return fmt.Errorf("--jsvm %q: use quickjs or goja", engine)
	}
}
