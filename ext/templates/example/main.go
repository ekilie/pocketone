// Command example is a PocketBase app with templates/v1 and one template set, "hello", served at /hello. Run
// `go run . template eject hello`, edit pb_templates/hello/page.tpl, and reload /hello.
package main

import (
	"log"
	"net/http"
	"testing/fstest"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	templatesv1 "pocketone.dev/ext/contracts/templates/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/templates"
)

func main() {
	app := pocketbase.New()
	if _, err := extkit.Setup(app, []extkit.Extension{templates.New(templates.Config{})}, nil, nil); err != nil {
		log.Fatal(err)
	}
	// an extension would go:embed its set; this one is inline
	templatesv1.Add(app, "hello", fstest.MapFS{"page.tpl": {Data: []byte(`<h1>Hello, {{.}}!</h1>`)}})
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/hello", func(e *core.RequestEvent) error {
			return templatesv1.Contract.Must(app).Render(e, "hello", "page", http.StatusOK, "world")
		})
		return se.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
