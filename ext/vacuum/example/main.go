// Command example is a PocketBase app that compacts its databases once when it starts serving (vacuum.App), the work
// the extension does on its admins' schedule. Run `go run . serve` and watch the log for "vacuum: once".
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/vacuum"
)

func main() {
	app := pocketbase.New()
	if _, err := extkit.Setup(app, []extkit.Extension{vacuum.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		go func() {
			run := vacuum.App(app)
			app.Logger().Info("vacuum: once", "saved_bytes", run.Saved, "skipped", run.Skipped, "error", run.Error)
		}()
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
