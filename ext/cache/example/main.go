// Command example is a PocketBase app with cache/v1 and one bucket, "example.slow". GET /slow takes a second the first
// time and answers from the cache for the next minute:
//
//	go run ./example serve --http=127.0.0.1:8090
//	curl http://127.0.0.1:8090/slow   # slow
//	curl http://127.0.0.1:8090/slow   # cached
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password, app user user@example.com / password.
package main

import (
	"log"
	"net/http"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"pocketone.dev/ext/cache"
	cachev1 "pocketone.dev/ext/contracts/cache/v1"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"
	if _, err := extkit.Setup(app, []extkit.Extension{cache.New(cache.Config{})}, nil, nil); err != nil {
		log.Fatal(err)
	}
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		b, err := cachev1.Open(app, "example.slow", cachev1.Options{MaxItems: 100, TTL: time.Minute})
		if err != nil {
			return err
		}
		se.Router.GET("/slow", func(e *core.RequestEvent) error {
			ctx := e.Request.Context()
			if v, ok, _ := b.Get(ctx, "answer"); ok {
				return e.String(http.StatusOK, "cached: "+string(v)+"\n")
			}
			time.Sleep(time.Second) // the expensive part
			v := time.Now().Format(time.RFC3339)
			_ = b.Set(ctx, "answer", []byte(v), 0)
			return e.String(http.StatusOK, "computed: "+v+"\n")
		})
		return se.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
