// Command example is a PocketOne app with remotes and API tokens: run two copies, add one as a remote of the other's
// project and sign in.
//
//	go run ./example serve --http=127.0.0.1:8090                # the "remote"
//	go run ./example remote add local http://127.0.0.1:8090    # in a project folder
//	go run ./example login local --email admin@example.com     # password: password; mints a pbt_ token
//	go run ./example remote whoami
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password, app user user@example.com / password.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/remotes"
	"github.com/pockethost/pocketone/ext/tokens"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"

	// registered through their contracts (the standard set does this for you): remotes provides remote/v1 (whoami at
	// /api/_c/remote/v1, status, restart, logs) and the remote/login/logout/link commands; tokens lets login mint a
	// revocable API token
	if _, err := extkit.Setup(app, []extkit.Extension{tokens.New(tokens.Config{}), remotes.New(remotes.Config{})}, nil, nil); err != nil {
		log.Fatal(err)
	}

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
