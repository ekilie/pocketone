// Command example is a PocketBase app whose restore/v1 comes from its backups: create a backup in the dashboard
// (Settings → Backups), then `curl -H "Authorization: <superuser token>" http://127.0.0.1:8090/api/_c/restore/v1`
// lists it, and POST /api/_c/restore/v1/restore with {"at": "<RFC 3339 time>"} restores the latest backup before it.
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"pocketone.dev/ext/backups"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"
	if _, err := extkit.Setup(app, []extkit.Extension{backups.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
