// Command example serves example/pb_hooks with the jsvm extension: one QuickJS VM for the app, so the hook file's
// top-level helpers and state are visible to every handler.
//
//	cd example && go run . serve
//	curl http://127.0.0.1:8090/hello/world
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/jsvm"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app)
	jsvm.MustRegister(app, jsvm.Config{HooksDir: "pb_hooks", MigrationsDir: "pb_migrations"})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
