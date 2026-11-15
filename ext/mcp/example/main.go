// Command example runs one PocketOne app with an MCP endpoint at /mcp: the built-in pb_* tools (the whole stock REST
// API), a tool over a Go route, and a tool over a JS route from pb_hooks/mcp.pb.js.
//
//	cd ext/mcp/example && go run . serve
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password, app user user@example.com / password.
//
// Sign-in is OAuth (ext/oauth-server + ext/tokens): add the endpoint to Claude Code and sign in in the browser when it
// asks. Tool calls run as whoever signed in.
//
//	claude mcp add --transport http pb-example http://127.0.0.1:8090/mcp
//	claude mcp login pb-example   (or /mcp in a session)
package main

import (
	"log"
	"net/http"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/dop251/goja"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/mcp"
	oauthserver "github.com/pockethost/pocketone/ext/oauth-server"
	"github.com/pockethost/pocketone/ext/tokens"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"

	// OAuth sign-in: API tokens + the authorization server. ext/mcp picks it up on serve and turns on dynamic client
	// registration.
	tokens.MustRegister(app, tokens.Config{})
	oauthserver.MustRegister(app, oauthserver.Config{})

	server := mcp.MustRegister(app, mcp.Config{
		Name:        "pb-example",
		Description: "Example PocketOne app with ext/mcp.",
	})

	// A tool from Go: write the REST endpoint, then the tool wrapper. The tool runs the request as the caller, so
	// the route's own auth check is what protects it.
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/example/whoami", func(e *core.RequestEvent) error {
			return e.JSON(http.StatusOK, map[string]any{"collection": e.Auth.Collection().Name, "email": e.Auth.Email()})
		}).Bind(apis.RequireAuth())
		return se.Next()
	})
	server.MustAddEndpoint(mcp.Endpoint{
		Name:        "example_get_whoami",
		Title:       "Who am I",
		Description: "The signed-in record's collection and email.",
		Method:      http.MethodGet,
		Path:        "/api/example/whoami",
	})

	jsvm.MustRegister(app, jsvm.Config{
		OnInit: func(vm *goja.Runtime) { mcp.BindJS(vm, app) },
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
