// Command example is a PocketBase app with settings/v1: the standard extensions' admin settings (here pitr's) behind
// one API. `curl -H "Authorization: <superuser token>" http://127.0.0.1:8090/api/_c/settings/v1` lists them.
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/pitr"
	"pocketone.dev/ext/settings"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"
	if _, err := extkit.Setup(app, []extkit.Extension{settings.New(), pitr.New(pitr.Config{})}, nil, nil); err != nil {
		log.Fatal(err)
	}
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
