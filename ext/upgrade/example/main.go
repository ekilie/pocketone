// Command example is a PocketOne app with just the upgrade command. A local build has no release version, so it is
// kept as "example.old"; release builds are kept as "<name>-<version>".
//
//	go run ./example upgrade
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"pocketone.dev/ext/upgrade"
)

func main() {
	app := pocketbase.New()
	upgrade.MustRegister(app, app.RootCmd, upgrade.Config{})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
