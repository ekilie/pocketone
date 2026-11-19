// Command example is a PocketBase app with only the desktop extension: `go run . desktop build` in an app folder
// (one with pb_hooks, pb_migrations or pb_public, like ./hello) builds it as a desktop app. The pocketone binary
// has the same command.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"

	clicmdv1 "pocketone.dev/ext/contracts/cli/commands/v1"
	"pocketone.dev/ext/desktop"
	"pocketone.dev/ext/extkit"
)

func main() {
	app := pocketbase.New()
	if _, err := extkit.Setup(app, []extkit.Extension{desktop.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	clicmdv1.Mount(app, app.RootCmd) // the commands extensions provide; the standard set does this for pocketone
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
