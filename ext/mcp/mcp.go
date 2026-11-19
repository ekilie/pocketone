// Package mcp serves a Model Context Protocol endpoint for one PocketBase app: a thin mapping of the app's REST API
// for agents (a Claude connector, Claude Code, any Streamable HTTP client).
//
//	app := pocketbase.New()
//	mcp.MustRegister(app, mcp.Config{})
//
// Every tool maps onto one REST endpoint. A call turns the tool's arguments into that request, runs it through the
// app's own router in-process as the caller (their PocketBase auth), and turns the response back into an MCP result.
// API rules, request hooks, middleware and rate limits apply because it is the REST request; ext/mcp makes no
// permission decisions of its own.
//
// The built-in pb_* tools map the whole stock API. Extensions add tools for their own routes with [Server.AddEndpoint]
// on [For](app); an app's pb_hooks add them with $mcp.addEndpoint (see [BindJS]). The shape is always the same:
// write the REST endpoint, then the tool wrapper.
//
// Sign-in is an [Authenticator]. With the OAuth server in ext/api-tokens on the app it's [OAuth]: OAuth 2.1 the way Claude
// connectors expect, so a user adds https://<host>/mcp to Claude and signs in in a browser. Without them it falls
// back to [PocketBaseTokenAuth] (pasted PocketBase auth tokens).
package mcp

import (
	"fmt"
	"net/http"
	"net/url"
	mcpv1 "pocketone.dev/ext/contracts/mcp/v1"
	"regexp"
	"sort"
	"strings"
	"sync"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/router"
	"pocketone.dev/ext/extkit"
)

const (
	storeKey = "mcp"

	// DefaultPath is where the endpoint is served: mcp/v1 fixes it at /mcp.
	DefaultPath = mcpv1.Path

	// DefaultMaxResponse caps a JSON tool result (1 MB).
	DefaultMaxResponse = 1 << 20
	// DefaultMaxFile caps a file sent to or read from a tool (10 MB).
	DefaultMaxFile = 10 << 20
)

// SupportedVersions are the MCP protocol revisions this server speaks, newest first.
var SupportedVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26"}

// Config is set by whoever builds the binary (or, with Pocker, per instance).
type Config struct {
	// Name is serverInfo.name (default: a slug of Settings → Application name, or "pocketbase").
	Name string
	// Version is serverInfo.version (default "dev").
	Version string
	// Description is serverInfo.description.
	Description string
	// Icons go to serverInfo.icons, e.g. [{"src": "https://…/logo.png", "mimeType": "image/png"}].
	Icons []map[string]any
	// Instructions are sent in initialize, before anything extensions and hooks add with [Server.AddInstructions].
	Instructions string

	// PublicURL is the origin clients reach the app at, used for the resource URL in protected resource metadata
	// (default: derived from the request).
	PublicURL string

	// Auth signs requests in. Default: [OAuth] when ext/api-tokens (with its OAuth server) is registered on the app (decided
	// on serve, so registration order doesn't matter), else [PocketBaseTokenAuth].
	Auth Authenticator

	// NoBuiltins leaves out the pb_* tools. BuiltinGroups limits them to some groups (see [BuiltinGroups]).
	NoBuiltins    bool
	BuiltinGroups []string
	// ReadOnlyBuiltins registers only the read-only pb_* tools.
	ReadOnlyBuiltins bool

	// MaxResponse caps a JSON result in bytes (default [DefaultMaxResponse]); a longer one is cut and says so.
	MaxResponse int
	// MaxFile caps one file uploaded or downloaded through a tool (default [DefaultMaxFile]).
	MaxFile int

	// AllowedOrigins lists browser origins (scheme://host[:port]) allowed besides the app's own host. Requests from
	// other origins get 403 (the spec's DNS rebinding guard). Server-to-server clients send no Origin and pass.
	AllowedOrigins []string
}

// Annotations are hints for clients (they treat them as untrusted unless they trust the server).
type Annotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

// ReadOnly reports whether the tool only reads.
func (a *Annotations) ReadOnly() bool { return a != nil && a.ReadOnlyHint != nil && *a.ReadOnlyHint }

func ptr(b bool) *bool { return &b }

// Common annotation sets. An endpoint without annotations gets one from its method (GET/HEAD [Read], DELETE
// [Destroy], others [Write]).
var (
	Read    = &Annotations{ReadOnlyHint: ptr(true)}
	Write   = &Annotations{ReadOnlyHint: ptr(false), DestructiveHint: ptr(false)}
	Destroy = &Annotations{ReadOnlyHint: ptr(false), DestructiveHint: ptr(true)}
)

// Endpoint is one tool: a mapping onto a REST endpoint of the app.
//
// By default `{name}` segments of Path are filled from the argument of the same name; the other arguments go to the
// query string (GET, HEAD, DELETE) or the JSON body (other methods). An argument holding a file (`{"name": …,
// "base64": …}`, or a list of them) makes the body multipart, the way PocketBase takes uploads. Request and Response
// reshape the call when the default doesn't fit.
type Endpoint struct {
	// Name: lowercase `<extension>_<verb>_<noun>`, 1–128 of A-Za-z0-9_.-, unique per app. `pb_` is the built-ins'.
	Name        string
	Title       string
	Description string

	// Method and Path of the REST endpoint, e.g. "PATCH", "/api/collections/{collection}/records/{id}".
	Method string
	Path   string

	// InputSchema is a JSON Schema object (default: no arguments). Arguments are checked against its required
	// properties, types and enums before the request is made.
	InputSchema map[string]any
	// OutputSchema, when set, describes structuredContent.
	OutputSchema map[string]any
	// Annotations (default: from the method).
	Annotations *Annotations

	// MaxResponse is how many bytes of the response the tool reads (default [Config.MaxResponse]); file tools raise it
	// to [Config.MaxFile].
	MaxResponse int

	// SuperuserOnly hides the tool from other callers in tools/list (the endpoint is superuser-only anyway). It's
	// presentation, not permission: a call still goes to the endpoint, which decides.
	SuperuserOnly bool

	// Request, when set, builds the request instead of the default mapping. It sees the caller but can't change who
	// the request runs as.
	Request func(c *Call) (*Request, error)
	// Response, when set, turns the response into the tool result (any JSON-able value, or a [Content] list). Return
	// an error for a tool failure the model can read. Without it, 2xx is the result and anything else an error.
	Response func(c *Call, res *Response) (any, error)
}

// Request is one REST request a tool makes.
type Request struct {
	Method string
	// Path is the URL path, e.g. "/api/collections/posts/records". Fill path parameters with [PathValue] (or
	// [Call.Path]), never by joining raw strings.
	Path   string
	Query  url.Values
	Header http.Header
	// Body is sent as JSON (nil: no body), or as `@jsonPayload` next to Files when there are any.
	Body any
	// Files are multipart file parts, keyed by field name (e.g. "avatar", "documents+").
	Files map[string][]File
}

// File is a file sent in a multipart request, or read from a response.
type File struct {
	Name string
	Type string
	Data []byte
}

// Response is what the REST endpoint answered.
type Response struct {
	Status int
	Header http.Header
	// Body is the response body, cut at the cap when Truncated.
	Body      []byte
	Truncated bool
}

// Content is an MCP content list; a Response func returns it to send images, text or files as they are.
type Content []map[string]any

// Call is one tool call.
type Call struct {
	App   core.App
	Event *core.RequestEvent
	Grant *Grant
	Tool  *Endpoint
	// Args are the tool arguments (never nil).
	Args map[string]any

	s *Server
}

// Auth is the signed-in record the call runs as.
func (c *Call) Auth() *core.Record {
	if c.Grant == nil {
		return nil
	}
	return c.Grant.Auth
}

// Do makes another REST request as the same caller (e.g. a file token before a download) and returns the response,
// capped at max bytes (0: the JSON cap).
func (c *Call) Do(req *Request, max int) (*Response, error) {
	if max <= 0 {
		max = c.s.cfg.MaxResponse
	}
	return c.s.send(c, req, max)
}

// Server is the MCP service of one app. Get it with [For].
type Server struct {
	app core.App
	cfg Config

	mu           sync.RWMutex
	tools        map[string]*Endpoint
	instructions []string

	// The app's router, captured on serve; the handler is built from it on first use (after every OnServe hook has
	// added its routes). Pocker and tests never set http.Server.Handler, so the router is the only reliable source.
	router  *router.Router[*core.RequestEvent]
	muxOnce sync.Once
	mux     http.Handler
	muxErr  error
}

var toolName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

// MustRegister is [Register] that panics on error.
func MustRegister(app core.App, cfg Config) *Server {
	s, err := Register(app, cfg)
	if err != nil {
		panic(err)
	}
	return s
}

// Register adds the MCP endpoint and protected resource metadata to app (on serve) and the built-in tools. Call it
// before app.Start(), or from pocker's SetupApp. Extensions registered later add their tools through [For].
func Register(app core.App, cfg Config) (*Server, error) {
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	if cfg.MaxResponse <= 0 {
		cfg.MaxResponse = DefaultMaxResponse
	}
	if cfg.MaxFile <= 0 {
		cfg.MaxFile = DefaultMaxFile
	}
	s := &Server{app: app, cfg: cfg, tools: map[string]*Endpoint{}}
	if !cfg.NoBuiltins {
		for _, t := range builtinTools(cfg.BuiltinGroups, cfg.ReadOnlyBuiltins, cfg.MaxFile) {
			if err := s.AddEndpoint(t); err != nil {
				return nil, err
			}
		}
	}
	if p, ok := app.Store().Get(pendingKey).(*pending); ok { // declared by pb_hooks before Register
		p.mu.Lock()
		for _, t := range p.tools {
			if err := s.AddEndpoint(t); err != nil {
				p.mu.Unlock()
				return nil, err
			}
		}
		for _, text := range p.instructions {
			s.AddInstructions(text)
		}
		p.tools, p.instructions = nil, nil
		p.mu.Unlock()
	}
	app.Store().Set(storeKey, s)
	extkit.SetToolLister(app, s) // lets checks such as the extension conformance test see the tools

	// mcp/v1: the endpoint at /mcp, its metadata, and the tools of every contract the app provides
	mcpv1.Contract.ProvideDefault(app, endpoint{s})
	mcpv1.Bind(app)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		s.setupAuth()
		s.router = se.Router
		return se.Next()
	})
	return s, nil
}

// For returns the MCP server registered on app, or nil.
func For(app core.App) *Server {
	s, _ := app.Store().Get(storeKey).(*Server)
	return s
}

// App returns the app this server belongs to.
func (s *Server) App() core.App { return s.app }

// Path returns the endpoint path (/mcp).
func (s *Server) Path() string { return DefaultPath }

// AddEndpoint registers a tool. It fails on an invalid or duplicate name, or a missing method or path.
func (s *Server) AddEndpoint(t Endpoint) error {
	if !toolName.MatchString(t.Name) {
		return fmt.Errorf("mcp: invalid tool name %q (1-128 of A-Za-z0-9_.-)", t.Name)
	}
	t.Method = strings.ToUpper(strings.TrimSpace(t.Method))
	if t.Request == nil && (t.Method == "" || !strings.HasPrefix(t.Path, "/")) {
		return fmt.Errorf("mcp: tool %q needs a method and a path starting with /", t.Name)
	}
	if t.Annotations == nil {
		t.Annotations = methodAnnotations(t.Method)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tools[t.Name]; ok {
		return fmt.Errorf("mcp: tool %q is already registered", t.Name)
	}
	s.tools[t.Name] = &t
	return nil
}

// MustAddEndpoint is [Server.AddEndpoint] that panics on error.
func (s *Server) MustAddEndpoint(t Endpoint) {
	if err := s.AddEndpoint(t); err != nil {
		panic(err)
	}
}

// RemoveTool unregisters a tool (no-op when absent).
func (s *Server) RemoveTool(name string) {
	s.mu.Lock()
	delete(s.tools, name)
	s.mu.Unlock()
}

func methodAnnotations(method string) *Annotations {
	switch method {
	case http.MethodGet, http.MethodHead:
		return Read
	case http.MethodDelete:
		return Destroy
	}
	return Write
}

// Tools returns the registered tools: the app's and extensions' own first, then the pb_* built-ins, each by name.
func (s *Server) Tools() []*Endpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Endpoint, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		bi, bj := isBuiltin(out[i].Name), isBuiltin(out[j].Name)
		if bi != bj {
			return bj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func isBuiltin(name string) bool { return strings.HasPrefix(name, "pb_") }

// ToolInfos implements [extkit.ToolLister]: its own tools, then mcp/v1's (contracts' and extensions').
func (s *Server) ToolInfos() []extkit.ToolInfo {
	var out []extkit.ToolInfo
	for _, t := range s.Tools() {
		out = append(out, extkit.ToolInfo{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	for _, t := range s.contractTools() {
		out = append(out, extkit.ToolInfo{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	return out
}

// contractTools are mcp/v1's tools this server serves next to its own (an own tool wins on a name clash). Before the
// app's contracts are sealed (during registration) there are none yet.
func (s *Server) contractTools() (out []mcpv1.Tool) {
	defer func() {
		if r := recover(); r != nil {
			s.app.Logger().Warn("mcp: contract tools unavailable", "error", fmt.Sprint(r))
			out = nil
		}
	}()
	for _, t := range mcpv1.Tools(s.app) {
		if s.tool(t.Name) == nil {
			out = append(out, t)
		}
	}
	return out
}

func (s *Server) contractTool(name string) (mcpv1.Tool, bool) {
	for _, t := range s.contractTools() {
		if t.Name == name {
			return t, true
		}
	}
	return mcpv1.Tool{}, false
}

// endpoint is ext/mcp's mcp/v1 provider.
type endpoint struct{ s *Server }

func (p endpoint) ServeMCP(e *core.RequestEvent) error {
	if e.Request.Method != http.MethodPost {
		return methodNotAllowed(e)
	}
	return p.s.handlePost(e)
}

func (p endpoint) Metadata(e *core.RequestEvent) map[string]any {
	return p.s.ProtectedResourceMetadata(e)
}

func (s *Server) tool(name string) *Endpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tools[name]
}

// AddInstructions appends a paragraph to the instructions sent in initialize (extensions describe their tools here).
func (s *Server) AddInstructions(text string) {
	if text = strings.TrimSpace(text); text == "" {
		return
	}
	s.mu.Lock()
	s.instructions = append(s.instructions, text)
	s.mu.Unlock()
}

func (s *Server) instructionsText() string {
	own := false
	for _, t := range s.Tools() {
		if !isBuiltin(t.Name) {
			own = true
			break
		}
	}
	if len(s.contractTools()) > 0 {
		own = true
	}
	extra := mcpv1.Instructions(s.app)
	s.mu.RLock()
	defer s.mu.RUnlock()
	parts := []string{}
	if t := strings.TrimSpace(s.cfg.Instructions); t != "" {
		parts = append(parts, t)
	} else {
		name := s.app.Settings().Meta.AppName
		if name == "" {
			name = "this PocketBase app"
		}
		parts = append(parts, "Tools for "+name+", a PocketBase app. Each tool is one call to the app's REST API, "+
			"made as you (your account's API rules apply). The pb_* tools are PocketBase's generic API: records, "+
			"files and auth for everyone; collections, settings, logs, crons, SQL and backups for superusers. "+
			"Filters and sorts use PocketBase syntax; pb_list_collections (superusers) shows the data model.")
	}
	if own {
		parts = append(parts, "This app has its own tools (the ones without the pb_ prefix). Use them when one fits "+
			"the task: they carry the app's meaning and checks, and may span several collections. Fall back to pb_* "+
			"tools for anything they don't cover.")
	}
	parts = append(parts, s.instructions...)
	parts = append(parts, extra...)
	return strings.Join(parts, "\n\n")
}

// publicOrigin is the scheme://host clients use to reach the app. The OAuth resource must match the URL the client
// actually uses, so the request's host wins over Settings → Application URL (which defaults to localhost:8090);
// the setting only supplies the scheme when its host is the request's.
func (s *Server) publicOrigin(e *core.RequestEvent) string {
	if u := strings.TrimRight(s.cfg.PublicURL, "/"); u != "" {
		return u
	}
	host := e.Request.Host
	if app, err := url.Parse(s.app.Settings().Meta.AppURL); err == nil && app.Host != "" && strings.EqualFold(app.Host, host) {
		return app.Scheme + "://" + app.Host
	}
	scheme := "http"
	if e.Request.TLS != nil || strings.EqualFold(e.Request.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + host
}

// ResourceURL is the canonical URL of the endpoint (the OAuth resource / token audience).
func (s *Server) ResourceURL(e *core.RequestEvent) string { return s.publicOrigin(e) + DefaultPath }

func methodNotAllowed(e *core.RequestEvent) error {
	e.Response.Header().Set("Allow", http.MethodPost)
	return e.JSON(http.StatusMethodNotAllowed, map[string]any{"message": "Use POST."})
}
