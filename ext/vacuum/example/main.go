// Command example is a PocketBase app that compacts its databases once when it starts serving (vacuum.App), the work
// the extension does on the `vacuum` section's schedule. Run `go run . serve` and watch the log for "vacuum: done".
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/vacuum"
)

func main() {
	app := pocketbase.New()
	if _, err := extkit.Setup(app, []extkit.Extension{vacuum.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		go func() {
			if err := vacuum.App(app); err != nil {
				app.Logger().Error("vacuum", "error", err)
			}
		}()
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
