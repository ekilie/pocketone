// Command example serves its JS hooks and public files from files embedded in the binary: no pb_hooks or pb_public
// folder on disk.
//
//	go run ./example serve
//	curl http://127.0.0.1:8090/hello
//	curl http://127.0.0.1:8090/
package main

import (
	"embed"
	"io/fs"
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"pocketone.dev/ext/bundle"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/jsvm"
)

//go:embed hooks public
var files embed.FS

func main() {
	hooks, err := fs.Sub(files, "hooks")
	if err != nil {
		log.Fatal(err)
	}
	public, err := fs.Sub(files, "public")
	if err != nil {
		log.Fatal(err)
	}
	app := pocketbase.New()
	devusers.Register(app)
	jsvm.MustRegister(app, jsvm.Config{Extensions: []any{bundle.Hooks(hooks)}})
	bundle.Public(app, public, true)
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
