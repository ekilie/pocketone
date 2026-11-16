// Command example is a PocketOne app that accepts deploys and can deploy: deploy/v1 (/api/_c/deploy/v1), remote/v1
// (restart, logs), API tokens, and the deploy/dev/pull/logs/restart commands.
//
//	go run ./example serve --http=127.0.0.1:8090                  # the remote
//	cd myproject && mkdir pb_hooks && echo 'routerAdd("GET", "/hi", (e) => e.string(200, "hi"))' > pb_hooks/main.pb.js
//	go run ./example link http://127.0.0.1:8090 --email admin@example.com   # password: password
//	go run ./example deploy
//	curl http://127.0.0.1:8090/hi
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password, app user user@example.com / password.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"github.com/pockethost/pocketone/ext/deploy"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/jsvm"
	"github.com/pockethost/pocketone/ext/remotes"
	"github.com/pockethost/pocketone/ext/tokens"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"

	// registered through their contracts (the standard set does this for you); deploys land next to pb_data
	exts := []extkit.Extension{tokens.New(tokens.Config{}), remotes.New(remotes.Config{}), deploy.NewExtension(deploy.Config{})}
	if _, err := extkit.Setup(app, exts, nil, nil); err != nil {
		log.Fatal(err)
	}
	jsvm.MustRegister(app, jsvm.Config{}) // deployed pb_hooks and pb_migrations

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
