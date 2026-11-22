// Command example is a PocketBase app with the key-value store (kv/v1): page code and pb_hooks get $kv, and
// superusers read and change values at /api/_c/kv/v1, e.g.
// `curl -H "Authorization: <superuser token>" "http://127.0.0.1:8090/api/_c/kv/v1/get?name=example/greeting"`.
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	kvv1 "pocketone.dev/ext/contracts/kv/v1"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/kv"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"
	if _, err := extkit.Setup(app, []extkit.Extension{kv.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	// a value from Go, kept until someone changes it
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		var greeting string
		if found, err := kvv1.Get(e.App, "example/greeting", &greeting); err != nil {
			return err
		} else if !found {
			if err := kvv1.Set(e.App, "example/greeting", "Hello from the key-value store"); err != nil {
				return err
			}
		}
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
