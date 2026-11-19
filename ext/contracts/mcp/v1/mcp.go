// Package mcpv1 is the mcp/v1 contract: the app's Model Context Protocol endpoint, so agents (a Claude connector,
// Claude Code, any Streamable HTTP client) can work with it. The endpoint stays at /mcp, the one path people paste:
// MCP versions itself in the protocol, and OAuth tokens are bound to the resource URL.
//
// The contract owns what every provider serves the same way: the routes (/mcp and its protected resource metadata
// under /.well-known/), and the tools. Tools come from three places: every wire contract the app provides (its
// operations, called through the contract, so the same scope check and hooks apply), tools extensions add with
// [AddTool], and the provider's own (ext/mcp's pb_* tools over the REST API). Swap the provider and the contract tools
// stay.
package mcpv1

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
	"pocketone.dev/ext/extkit"
)

// Path is where the endpoint is served.
const Path = "/mcp"

// MetadataPath is its protected resource metadata (RFC 9728); the bare well-known path answers too.
const MetadataPath = "/.well-known/oauth-protected-resource" + Path

// Endpoint is an app's MCP server.
type Endpoint interface {
	// ServeMCP answers one request to /mcp (Streamable HTTP: POST; GET and DELETE answer 405 without sessions).
	ServeMCP(e *core.RequestEvent) error
	// Metadata is the protected resource metadata for /mcp: where clients sign in.
	Metadata(e *core.RequestEvent) map[string]any
}

// Contract is mcp/v1.
var Contract = extkit.NewContract[Endpoint]("mcp/v1", extkit.One)

const boundKey = "mcp/v1.bound"

// Bind serves the provider's endpoint at /mcp and its metadata on app. Providers call it in Register; it binds once.
func Bind(app core.App) {
	if app.Store().Has(boundKey) {
		return
	}
	app.Store().Set(boundKey, true)
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Id: "__mcp/v1__",
		Func: func(se *core.ServeEvent) error {
			ep := func(e *core.RequestEvent) (Endpoint, error) {
				p, ok := Contract.Of(e.App)
				if !ok {
					return nil, e.NotFoundError("", nil)
				}
				return p, nil
			}
			serve := func(e *core.RequestEvent) error {
				p, err := ep(e)
				if err != nil {
					return err
				}
				return p.ServeMCP(e)
			}
			// The provider answers an unknown or expired token itself, with the WWW-Authenticate challenge MCP clients
			// need to sign in again; PocketBase's catch-all 401 for unknown tokens would leave that header out.
			reject := apis.DefaultRejectUnknownTokenMiddlewareId
			se.Router.POST(Path, serve).Unbind(reject)
			se.Router.GET(Path, serve).Unbind(reject)
			se.Router.DELETE(Path, serve).Unbind(reject)
			meta := func(e *core.RequestEvent) error {
				p, err := ep(e)
				if err != nil {
					return err
				}
				e.Response.Header().Set("Access-Control-Allow-Origin", "*")
				return e.JSON(http.StatusOK, p.Metadata(e))
			}
			// public metadata: a stale token on the request mustn't hide where to sign in
			se.Router.GET("/.well-known/oauth-protected-resource", meta).Unbind(reject)
			se.Router.GET(MetadataPath, meta).Unbind(reject)
			return se.Next()
		},
	})
}

// Tool is a tool an extension adds to the app's MCP server, whoever provides it.
type Tool struct {
	// Name: lowercase `<extension>_<verb>_<noun>`, unique per app.
	Name        string
	Title       string
	Description string
	// InputSchema is a JSON Schema object for the arguments (nil: none).
	InputSchema map[string]any
	ReadOnly    bool
	Destructive bool
	// Scope is the token scope a connection limited to scopes needs to see and call it ("" for any).
	Scope string
	// SuperuserOnly hides it from other callers in tools/list (presentation: Call still decides).
	SuperuserOnly bool
	// Call runs it as a (the signed-in caller, as extkit.Auth); the result is any JSON-able value.
	Call func(ctx context.Context, a extkit.Auth, args json.RawMessage) (any, error)
}

const storeKey = "mcp/v1.registry"

type registry struct {
	mu           sync.Mutex
	tools        map[string]Tool
	instructions []string
}

func registryOf(app core.App) *registry {
	return app.Store().GetOrSet(storeKey, func() any { return &registry{tools: map[string]Tool{}} }).(*registry)
}

// AddTool adds a tool to app's MCP server (call it in Register). A second tool with the same name replaces the first.
func AddTool(app core.App, t Tool) {
	r := registryOf(app)
	r.mu.Lock()
	r.tools[t.Name] = t
	r.mu.Unlock()
}

// AddInstructions adds a paragraph to what the server tells clients when they connect (what your tools are for).
func AddInstructions(app core.App, text string) {
	r := registryOf(app)
	r.mu.Lock()
	r.instructions = append(r.instructions, text)
	r.mu.Unlock()
}

// Instructions are the paragraphs extensions added.
func Instructions(app core.App) []string {
	r := registryOf(app)
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.instructions...)
}

// Tools are the tools every provider serves on app, by name: every provided wire contract's operations, then the
// ones extensions added (which win on a name clash).
func Tools(app core.App) []Tool {
	byName := map[string]Tool{}
	for _, t := range extkit.ProvidedTools(app) {
		byName[t.Name] = Tool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema, ReadOnly: t.ReadOnly,
			Scope: t.Scope, SuperuserOnly: t.Scope != extkit.ScopePublic, Call: t.Call} // contract operations other than public ones need a superuser
	}
	r := registryOf(app)
	r.mu.Lock()
	for name, t := range r.tools {
		byName[name] = t
	}
	r.mu.Unlock()
	out := make([]Tool, 0, len(byName))
	for _, t := range byName {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
