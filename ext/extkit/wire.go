package extkit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
)

// ContractPrefix is where wire contracts are served: /api/_c/<id>/<path>.
const ContractPrefix = "/api/_c/"

// Scopes with a special meaning in an operation's scope.
const (
	ScopePublic = "public" // no credential needed
	ScopeAny    = "any"    // any valid credential, whatever its scopes
)

// Auth is who a contract call acts as, however it arrived.
type Auth struct {
	Kind   string   // "superuser", "record", "token", "host", or "" (anonymous)
	ID     string   // record or token id
	Scopes []string // a token's scopes; nil means every scope (superuser, record JWT, host)
	Via    string   // "http", "mcp", "go", "host": for logs only, never for checks

	// Collection, RecordID and Email are the auth record the credential acts as (a token's owner), when there is one.
	Collection string
	RecordID   string
	Email      string

	host bool // authenticated by a multisite host's own auth (HostHandler), which decides who may call
}

// Internal is the auth an extension uses to call a contract in the same app for its own work (bulk mail checking
// the suppression list before it sends): a superuser's rights, logged as "internal". Never use it for a call made
// on someone's behalf; pass their Auth on instead.
var Internal = Auth{Kind: "superuser", Via: "internal"}

// Superuser reports whether a acts as a superuser: a superuser's JWT, an API token a superuser owns, or a call a
// multisite host authenticated for its instance. Contract operations other than public ones need it.
func (a Auth) Superuser() bool {
	return a.Kind == "superuser" || a.host || (a.Kind == "token" && a.Collection == core.CollectionNameSuperusers)
}

// Has reports whether a holds scope. Anonymous holds none; nil Scopes hold all.
func (a Auth) Has(scope string) bool {
	if a.Kind == "" {
		return false
	}
	if a.Scopes == nil || scope == "" || scope == ScopeAny {
		return true
	}
	for _, s := range a.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// Error is a contract's typed error. Each surface maps it: HTTP status, MCP tool error, CLI message.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Errors every contract can return.
var (
	ErrUnauthorized = &Error{Code: "unauthorized", Message: "sign in first", Status: http.StatusUnauthorized}
	ErrForbidden    = &Error{Code: "forbidden", Message: "this credential lacks the operation's scope", Status: http.StatusForbidden}
	ErrNotSuperuser = &Error{Code: "forbidden", Message: "only superusers may call this", Status: http.StatusForbidden}
)

// RawIn is an operation input that is a request body (an upload), not JSON.
type RawIn struct {
	Query       url.Values
	Body        io.Reader
	ContentType string
	Header      http.Header // the request's headers (a webhook's signature or secret); nil when called from Go
}

// RawOut is an operation result that is a response body (a download), not JSON.
type RawOut struct {
	Body        io.ReadCloser
	ContentType string
	Status      int // the HTTP status; 0 means 200
}

// Stream is an operation result that keeps sending entries: the route sends them as server-sent events, one
// `event: <op>` per entry, the initial ones first, then either what Run pushes as it happens (an answer being written)
// or whatever Poll returns (a log being tailed), until it's over or the client goes away.
type Stream struct {
	Initial []any
	Poll    func() []any // called every StreamPoll; nil = only the initial entries (ignored when Run is set)
	// Run, when set, pushes entries as they happen: it calls send for each, in order, and returns when the stream is
	// over. send fails once the receiver is gone; Run should then stop. ctx ends when the receiver goes away. A Run
	// error reaches HTTP clients as a final `event: error` with the contract error. In the same process, call Run
	// yourself with your own send.
	Run   func(ctx context.Context, send func(entry any) error) error
	Close func()
	// Done, on a client's stream, closes when the server ends it (or the connection drops). Nil on a server's.
	Done <-chan struct{}
	// Err, on a client's stream, is the error the server ended it with (its `event: error`), once Done is closed.
	Err func() error
}

// StreamPoll and StreamPing pace a stream: how often Poll runs, and how often a comment keeps idle connections open.
var (
	StreamPoll = 500 * time.Millisecond
	StreamPing = 25 * time.Second
)

// OutKind is what an operation returns.
type OutKind int

const (
	OutJSON OutKind = iota
	OutRaw
	OutStream
)

// OpRequest is an operation's input as it arrived.
type OpRequest struct {
	JSON        json.RawMessage // the body, for JSON operations with a body
	Query       url.Values
	Body        io.Reader // the body, for RawIn operations
	ContentType string
	Header      http.Header
}

// Op is one operation of a wire contract: METHOD /api/_c/<id>/<Path>, an MCP tool (JSON operations), a client call.
type Op[T any] struct {
	Name   string // operation name: the hook's (OnUpload), the tool's (deploy_upload)
	Desc   string
	Method string       // default POST
	Path   string       // under the contract's prefix; "" is the prefix itself
	Scope  string       // the token scope it needs ("" = the wire's; ScopePublic, ScopeAny)
	RawIn  bool         // the input is the request body (RawIn)
	In     reflect.Type // the input's type (nil: none, or RawIn), for clients that build it (the CLI's flags)
	Out    OutKind
	Invoke func(ctx context.Context, impl T, a Auth, req OpRequest) (any, error)
}

func (op *Op[T]) method() string {
	if op.Method == "" {
		return http.MethodPost
	}
	return op.Method
}

// Wire is a contract with an HTTP API, action hooks, MCP tools and a client, all from one list of ops.
type Wire[T any] struct {
	C     *Contract[T]
	Scope string // the default token scope of its operations
	Ops   []Op[T]

	mu    sync.Mutex
	hooks map[any]map[string]*hook.Hook[*CallEvent] // owner (core.App or host *Scope) → op → hook
}

// CallEvent is one contract call passing through its action hooks.
type CallEvent struct {
	hook.Event
	Contract ContractID
	Op       string
	Auth     Auth
	Input    any // pointer to the op's input; hooks may change it
}

// NewWire declares a wire contract. guarded builds the outermost layer: a T whose methods call [Run].
func NewWire[T any](c *Contract[T], scope string, guarded func(inner T, g *Guard[T]) T, ops ...Op[T]) *Wire[T] {
	if c.card != One {
		panic("extkit: wire contracts are One")
	}
	w := &Wire[T]{C: c, Scope: scope, Ops: ops, hooks: map[any]map[string]*hook.Hook[*CallEvent]{}}
	c.guard = func(owner any, inner T) T { return guarded(inner, &Guard[T]{w: w, owner: owner}) }
	wiresMu.Lock()
	wires = append(wires, w)
	wiresMu.Unlock()
	names := make([]string, len(ops))
	for i, op := range ops {
		names[i] = op.Name
	}
	bindJSHooks(c.id, names, w.On)
	return w
}

func (w *Wire[T]) op(name string) *Op[T] {
	for i := range w.Ops {
		if w.Ops[i].Name == name {
			return &w.Ops[i]
		}
	}
	return nil
}

func (w *Wire[T]) scopeOf(op *Op[T]) string {
	if op != nil && op.Scope != "" {
		return op.Scope
	}
	return w.Scope
}

// On returns the action hook of op for owner (an app, or a host scope for calls the host answers).
func (w *Wire[T]) On(owner any, op string) *hook.Hook[*CallEvent] {
	w.mu.Lock()
	defer w.mu.Unlock()
	m := w.hooks[owner]
	if m == nil {
		m = map[string]*hook.Hook[*CallEvent]{}
		w.hooks[owner] = m
	}
	h := m[op]
	if h == nil {
		h = &hook.Hook[*CallEvent]{}
		m[op] = h
	}
	return h
}

// mounter is a wire contract, as Finish sees it: mounted on an app whose seal gave it a provider.
type mounter interface {
	id() ContractID
	info() WireInfo
	scopes() []WireScope
	provided(app core.App) bool
	provider(app core.App) (string, bool)
	Mount(app core.App, se *core.ServeEvent)
	tools(app core.App) []Tool
}

// ProvidedTools are the MCP tools of every wire contract app provides: what an MCP server (mcp/v1) offers besides its
// own. Each calls the contract, so it passes the same scope check and hooks as the HTTP API.
func ProvidedTools(app core.App) []Tool {
	var out []Tool
	for _, w := range providedWires(app) {
		out = append(out, w.tools(app)...)
	}
	return out
}

func (w *Wire[T]) tools(app core.App) []Tool { return w.Tools(app) }

var (
	wiresMu sync.Mutex
	wires   []mounter
)

func (w *Wire[T]) provided(app core.App) bool {
	_, ok := w.C.Of(app)
	return ok
}

// Provided lists the wire contracts app offers (after the seal), e.g. for whoami.
func Provided(app core.App) []ContractID {
	wiresMu.Lock()
	ws := append([]mounter(nil), wires...)
	wiresMu.Unlock()
	var out []ContractID
	for _, w := range ws {
		if w.provided(app) {
			out = append(out, w.id())
		}
	}
	return out
}

func (w *Wire[T]) id() ContractID { return w.C.id }

func (w *Wire[T]) provider(app core.App) (string, bool) { return w.C.Provider(app) }

// APIInfo is a contract linked into the binary that has an HTTP API under its Prefix: a wire contract's generated
// operations, or a routed contract's own API (Routed, no Ops). Provider names the extension that answers it on the app
// it was read from ("" with Provided false: nothing does there).
type APIInfo struct {
	ID       ContractID
	Prefix   string
	Routed   bool
	Ops      []OpInfo
	Provider string
	Provided bool
}

// APIs describes every contract with an HTTP API linked into the binary, by id, and who provides each on app (after
// the seal). Generated docs read it (`pocketone reference --json`).
func APIs(app core.App) []APIInfo {
	wiresMu.Lock()
	ws := append([]mounter(nil), wires...)
	wiresMu.Unlock()
	out := make([]APIInfo, 0, len(ws))
	for _, w := range ws {
		wi := w.info()
		_, routed := w.(interface{ isRouted() })
		ai := APIInfo{ID: w.id(), Prefix: opPath(w.id(), ""), Routed: routed, Ops: wi.Ops}
		if app != nil {
			ai.Provider, ai.Provided = w.provider(app)
		}
		out = append(out, ai)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ToolName is the MCP tool of operation op of contract id: the id's name with dots as underscores, then the op
// (deploy/v1's check is deploy_check, mail.suppression/v1's check is mail_suppression_check).
func ToolName(id ContractID, op string) string {
	return strings.ReplaceAll(strings.SplitN(string(id), "/", 2)[0], ".", "_") + "_" + op
}

// HasTool says whether an operation gets an MCP tool: uploads and downloads don't.
func (op OpInfo) HasTool() bool { return !op.RawIn && op.Out != OutRaw }

func (w *Wire[T]) info() WireInfo {
	wi := WireInfo{ID: w.C.id, Ops: make([]OpInfo, len(w.Ops))}
	for i, op := range w.Ops {
		scope := op.Scope
		if scope == "" {
			scope = w.Scope
		}
		wi.Ops[i] = OpInfo{Name: op.Name, Desc: op.Desc, Method: op.method(), Path: op.Path, Scope: scope,
			RawIn: op.RawIn, In: op.In, Out: op.Out}
	}
	return wi
}

// WireInfo describes a wire contract's operations, for clients built from them (the CLI's commands).
type WireInfo struct {
	ID  ContractID
	Ops []OpInfo
}

// OpInfo is one operation: METHOD /api/_c/<id>/<Path>.
type OpInfo struct {
	Name   string // snake case: restore_to
	Desc   string
	Method string
	Path   string
	Scope  string // the token scope it needs (ScopePublic: anyone)
	RawIn  bool
	In     reflect.Type // nil: no input (or RawIn)
	Out    OutKind
}

// Wires describes every wire contract linked into the binary, by id. Routed contracts (their own API shape) aren't
// listed.
func Wires() []WireInfo {
	wiresMu.Lock()
	ws := append([]mounter(nil), wires...)
	wiresMu.Unlock()
	var out []WireInfo
	for _, w := range ws {
		if wi := w.info(); len(wi.Ops) > 0 {
			out = append(out, wi)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// NewCaller calls the wire contract id on a remote app (for clients that don't import the contract's package).
func NewCaller(id ContractID, baseURL, token string, hc *http.Client) *Caller {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Caller{BaseURL: strings.TrimSuffix(baseURL, "/"), Token: token, HTTP: hc, id: id}
}

// WireScope is a token scope wire contracts' operations need, and the paths it reaches.
type WireScope struct {
	Name        string
	Description string
	Paths       []string
}

// ProvidedScopes lists the token scopes of the wire contracts app offers (after the seal), for a token service to
// register on that app.
func ProvidedScopes(app core.App) []WireScope {
	var out []WireScope
	for _, w := range providedWires(app) {
		out = append(out, w.scopes()...)
	}
	return out
}

// WireScopes lists the token scopes of every wire contract linked into the binary (not ScopePublic or ScopeAny), so
// a token service (ext/api-tokens) can offer and enforce them.
func WireScopes() []WireScope {
	wiresMu.Lock()
	ws := append([]mounter(nil), wires...)
	wiresMu.Unlock()
	byName := map[string]*WireScope{}
	var order []string
	for _, w := range ws {
		for _, s := range w.scopes() {
			ex := byName[s.Name]
			if ex == nil {
				byName[s.Name] = &WireScope{Name: s.Name, Description: s.Description}
				ex = byName[s.Name]
				order = append(order, s.Name)
			}
			ex.Paths = append(ex.Paths, s.Paths...)
		}
	}
	out := make([]WireScope, 0, len(order))
	for _, n := range order {
		out = append(out, *byName[n])
	}
	return out
}

func (w *Wire[T]) scopes() []WireScope {
	byName := map[string]*WireScope{}
	var order []string
	for i := range w.Ops {
		op := &w.Ops[i]
		name := w.scopeOf(op)
		if name == "" || name == ScopePublic || name == ScopeAny {
			continue
		}
		s := byName[name]
		if s == nil {
			s = &WireScope{Name: name}
			byName[name] = s
			order = append(order, name)
		}
		s.Paths = append(s.Paths, opPath(w.C.id, op.Path))
		if s.Description == "" {
			s.Description = string(w.C.id) + ":"
		}
		s.Description += " " + op.Name
	}
	out := make([]WireScope, 0, len(order))
	for _, n := range order {
		out = append(out, *byName[n])
	}
	return out
}

func providedWires(app core.App) []mounter {
	wiresMu.Lock()
	ws := append([]mounter(nil), wires...)
	wiresMu.Unlock()
	var mine []mounter
	for _, w := range ws {
		if w.provided(app) {
			mine = append(mine, w)
		}
	}
	return mine
}

var (
	uisMu sync.Mutex
	uis   = map[ContractID]core.UIExtension{}
)

// UI gives the contract a dashboard screen (an admin extension: main.js and friends in fsys), shown on every app that
// provides the contract, whichever extension provides it. The screen calls the contract's API, so it works the same
// for every provider. Call it where the contract is declared.
func (c *Contract[T]) UI(name string, fsys fs.FS) *Contract[T] {
	uisMu.Lock()
	uis[c.id] = core.UIExtension{Name: name, FS: fsys}
	uisMu.Unlock()
	return c
}

// mountUIs adds the dashboard screens of the contracts app provides.
func mountUIs(app core.App, se *core.ServeEvent) {
	uisMu.Lock()
	defer uisMu.Unlock()
	if len(uis) == 0 {
		return
	}
	for _, id := range Provided(app) {
		if u, ok := uis[id]; ok {
			se.UIExtensions = append(se.UIExtensions, u)
		}
	}
}

// mountWires binds every wire contract that app's seal gave a provider to app's router.
func mountWires(app core.App) {
	mine := providedWires(app)
	if len(mine) == 0 {
		return
	}
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		for _, w := range mine {
			w.Mount(app, se)
		}
		mountUIs(app, se)
		return se.Next()
	})
}

// DecodeInput decodes a call's JSON input into in; an empty body leaves in at its zero value.
func DecodeInput(raw json.RawMessage, in any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, in); err != nil {
		return &Error{Code: "bad_request", Message: err.Error(), Status: http.StatusBadRequest}
	}
	return nil
}

// DecodeQuery fills the struct out points at from query parameters named by its json tags (strings, bools, numbers,
// string slices).
func DecodeQuery(q url.Values, out any) error {
	v := reflect.ValueOf(out).Elem()
	t := v.Type()
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("extkit: DecodeQuery needs a pointer to a struct, got %s", t)
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" {
			name = f.Name
		}
		vals, ok := q[name]
		if !ok || len(vals) == 0 || name == "-" {
			continue
		}
		fv := v.Field(i)
		bad := func(err error) error {
			return &Error{Code: "bad_request", Message: fmt.Sprintf("%s: %v", name, err), Status: http.StatusBadRequest}
		}
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(vals[0])
		case reflect.Bool:
			b, err := strconv.ParseBool(vals[0])
			if err != nil {
				return bad(err)
			}
			fv.SetBool(b)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(vals[0], 10, 64)
			if err != nil {
				return bad(err)
			}
			fv.SetInt(n)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(vals[0], 10, 64)
			if err != nil {
				return bad(err)
			}
			fv.SetUint(n)
		case reflect.Float32, reflect.Float64:
			n, err := strconv.ParseFloat(vals[0], 64)
			if err != nil {
				return bad(err)
			}
			fv.SetFloat(n)
		case reflect.Slice:
			if fv.Type().Elem().Kind() == reflect.String {
				fv.Set(reflect.ValueOf(append([]string(nil), vals...)))
			}
		}
	}
	return nil
}

// EncodeQuery is DecodeQuery's inverse, for clients.
func EncodeQuery(in any) url.Values {
	q := url.Values{}
	if in == nil {
		return q
	}
	v := reflect.Indirect(reflect.ValueOf(in))
	if v.Kind() != reflect.Struct {
		return q
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" {
			name = f.Name
		}
		if name == "-" || v.Field(i).IsZero() {
			continue
		}
		fv := v.Field(i)
		if fv.Kind() == reflect.Slice {
			for j := 0; j < fv.Len(); j++ {
				q.Add(name, fmt.Sprint(fv.Index(j).Interface()))
			}
			continue
		}
		q.Set(name, fmt.Sprint(fv.Interface()))
	}
	return q
}

// Guard is handed to a contract's guarded wrapper; [Run] does the scope check and action hooks.
type Guard[T any] struct {
	w     *Wire[T]
	owner any
}

// Run is the outermost layer of every call: scope check, then the op's action hooks, then call (the next layer).
func Run[T, In, Out any](g *Guard[T], a Auth, op string, in *In, call func(*In) (Out, error)) (Out, error) {
	var out Out
	scope := g.w.scopeOf(g.w.op(op))
	if scope != ScopePublic {
		if a.Kind == "" {
			return out, ErrUnauthorized
		}
		if !a.Superuser() {
			return out, ErrNotSuperuser
		}
		if !a.Has(scope) {
			return out, ErrForbidden
		}
	}
	e := &CallEvent{Contract: g.w.C.id, Op: op, Auth: a, Input: in}
	err := g.w.On(g.owner, op).Trigger(e, func(e *CallEvent) error {
		var err error
		out, err = call(e.Input.(*In))
		return err
	})
	return out, err
}

// ---------------------------------------------------------------------------------------------------------------
// HTTP: in an app, and in front of a host's runtime.

func opPath(id ContractID, op string) string {
	p := ContractPrefix + string(id)
	if op != "" {
		p += "/" + op
	}
	return p
}

// Mount serves the contract inside app at /api/_c/<id>/, calling whoever provides it after the seal. Finish mounts
// every wire contract the seal gave a provider; call it yourself only on a router you build. Auth comes from the
// request: PocketBase's own auth, and ext/api-tokens' API tokens (their scopes).
func (w *Wire[T]) Mount(app core.App, se *core.ServeEvent) {
	for _, op := range w.Ops {
		op := op
		route := se.Router.Route(op.method(), opPath(w.C.id, op.Path), func(e *core.RequestEvent) error {
			impl, ok := w.C.Of(app)
			if !ok {
				return e.NotFoundError(fmt.Sprintf("this app doesn't provide %s", w.C.id), nil)
			}
			a := authOf(e)
			req, err := readOpRequest(e.Request, op.RawIn)
			if err == nil {
				var out any
				out, err = op.Invoke(e.Request.Context(), impl, a, req)
				if err == nil {
					return writeOut(e.Response, e.Request, op.Name, op.Out, out)
				}
			}
			return writeError(e.Response, err)
		})
		if op.RawIn {
			route.Unbind("pbBodyLimit") // uploads set their own limits
		}
	}
}

func readOpRequest(r *http.Request, rawIn bool) (OpRequest, error) {
	req := OpRequest{Query: r.URL.Query(), ContentType: r.Header.Get("Content-Type"), Header: r.Header}
	if rawIn {
		req.Body = r.Body
		return req, nil
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Body != nil {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return req, err
		}
		req.JSON = b
	}
	return req, nil
}

// authOf reads who the request acts as: a PocketBase auth record, and when ext/api-tokens authenticated it, the token's
// id and scopes.
func authOf(e *core.RequestEvent) Auth {
	if e.Auth == nil {
		return Auth{Via: "http"}
	}
	a := Auth{Kind: "record", ID: e.Auth.Id, Via: "http",
		Collection: e.Auth.Collection().Name, RecordID: e.Auth.Id, Email: e.Auth.Email()}
	if e.Auth.IsSuperuser() {
		a.Kind = "superuser"
	}
	if id, _ := e.Get("api-tokens.id").(string); id != "" {
		a.Kind, a.ID = "token", id
		a.Scopes, _ = e.Get("api-tokens.scopes").([]string)
		if a.Scopes != nil && len(a.Scopes) == 0 {
			a.Scopes = nil // an unscoped token has the owner's full rights
		}
	}
	return a
}

// HostHandler answers the contract for a host's instances, in front of the runtime (on a multisite host: one of its
// InstanceRoutes, after the gates, with multisite.InstanceID as instance): requests under /api/_c/<id>/ on
// an instance's hostname are served by the host's provider for that instance; everything else goes to next. instance
// maps a request to its instance id; auth authenticates it at the host (stock images have no ext/api-tokens).
func (w *Wire[T]) HostHandler(host *Scope, instance func(*http.Request) (string, bool), auth func(*http.Request) Auth, next http.Handler) http.Handler {
	root := opPath(w.C.id, "")
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if (r.URL.Path != root && !strings.HasPrefix(r.URL.Path, root+"/")) || !host.HostProvides(w.C.id) {
			next.ServeHTTP(rw, r)
			return
		}
		inst, ok := instance(r)
		if !ok {
			next.ServeHTTP(rw, r)
			return
		}
		sub := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, root), "/")
		for _, op := range w.Ops {
			if op.Path != sub || op.method() != r.Method {
				continue
			}
			impl, _ := w.C.ForInstance(host, inst)
			guarded := w.C.guardFor(host, impl)
			a := auth(r)
			a.Via = "host"
			a.host = a.Kind != "" // the host's auth accepted it
			req, err := readOpRequest(r, op.RawIn)
			if err == nil {
				var out any
				if out, err = op.Invoke(r.Context(), guarded, a, req); err == nil {
					_ = writeOut(rw, r, op.Name, op.Out, out)
					return
				}
			}
			_ = writeError(rw, err)
			return
		}
		http.NotFound(rw, r)
	})
}

// guardFor wraps v with the guard, owned by a host scope instead of an app.
func (c *Contract[T]) guardFor(owner any, v T) T {
	if c.guard == nil {
		return v
	}
	return c.guard(owner, v) // hooks owned by the host scope, not an app
}

func writeError(rw http.ResponseWriter, err error) error {
	var ce *Error
	if !errors.As(err, &ce) {
		ce = &Error{Code: "internal", Message: err.Error(), Status: http.StatusInternalServerError}
	}
	status := ce.Status
	if status == 0 {
		status = http.StatusBadRequest
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	return json.NewEncoder(rw).Encode(ce)
}

func writeOut(rw http.ResponseWriter, r *http.Request, op string, kind OutKind, out any) error {
	switch kind {
	case OutRaw:
		ro, _ := out.(RawOut)
		if ro.Body == nil {
			rw.WriteHeader(http.StatusNoContent)
			return nil
		}
		defer ro.Body.Close()
		if ro.ContentType != "" {
			rw.Header().Set("Content-Type", ro.ContentType)
		}
		if ro.Status != 0 {
			rw.WriteHeader(ro.Status)
		}
		_, err := io.Copy(rw, ro.Body)
		return err
	case OutStream:
		s, _ := out.(Stream)
		return writeStream(rw, r, op, s)
	}
	rw.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(rw).Encode(out)
}

// writeStream sends s as server-sent events until the client goes away.
func writeStream(w http.ResponseWriter, r *http.Request, event string, s Stream) error {
	if s.Close != nil {
		defer s.Close()
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=UTF-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	rc := http.NewResponseController(w)
	// a stream outlives the server's WriteTimeout (PocketBase: 5 min), which would cut it with an unexpected EOF; clear
	// the deadline, as PocketBase's realtime does
	_ = rc.SetWriteDeadline(time.Time{})
	w.WriteHeader(http.StatusOK)
	send := func(entries []any) error {
		for _, e := range entries {
			b, err := json.Marshal(e)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
				return err
			}
		}
		return rc.Flush()
	}
	if err := send(s.Initial); err != nil {
		return err
	}
	if s.Run != nil {
		err := s.Run(r.Context(), func(e any) error { return send([]any{e}) })
		if err != nil && r.Context().Err() == nil {
			var ce *Error
			if !errors.As(err, &ce) {
				ce = &Error{Code: "internal", Message: err.Error(), Status: http.StatusInternalServerError}
			}
			b, _ := json.Marshal(ce)
			_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", b)
			_ = rc.Flush()
		}
		return nil
	}
	if s.Poll == nil {
		return nil
	}
	poll := time.NewTicker(StreamPoll)
	ping := time.NewTicker(StreamPing)
	defer poll.Stop()
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-poll.C:
			if err := send(s.Poll()); err != nil {
				return nil
			}
		case <-ping.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return nil
			}
			_ = rc.Flush()
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------
// Client: the same ops over HTTP, for the CLI.

// Caller calls a wire contract on a remote app.
type Caller struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	id      ContractID
}

// Client returns a caller for the contract on baseURL.
func (w *Wire[T]) Client(baseURL, token string, hc *http.Client) *Caller {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Caller{BaseURL: strings.TrimSuffix(baseURL, "/"), Token: token, HTTP: hc, id: w.C.id}
}

func (c *Caller) do(ctx context.Context, method, path string, q url.Values, body io.Reader, contentType string) (*http.Response, error) {
	u := c.BaseURL + opPath(c.id, path)
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", c.Token) // PocketBase takes a token with or without "Bearer "
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		defer res.Body.Close()
		ce := &Error{Status: res.StatusCode}
		if json.NewDecoder(res.Body).Decode(ce) != nil || ce.Code == "" {
			return nil, &Error{Code: "http", Message: fmt.Sprintf("%s %s: HTTP %d", method, opPath(c.id, path), res.StatusCode), Status: res.StatusCode}
		}
		return nil, ce
	}
	return res, nil
}

// Call sends in to an operation and decodes the result into out (nil to ignore it). GET and DELETE send in as query
// parameters, other methods as a JSON body. A contract error comes back as *Error.
func (c *Caller) Call(ctx context.Context, method, path string, in, out any) error {
	var q url.Values
	var body io.Reader
	ct := ""
	if method == http.MethodGet || method == http.MethodDelete || method == http.MethodHead {
		q = EncodeQuery(in)
	} else {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body, ct = bytes.NewReader(b), "application/json"
	}
	res, err := c.do(ctx, method, path, q, body, ct)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// CallRawIn sends in's body (an upload) and decodes the JSON result into out.
func (c *Caller) CallRawIn(ctx context.Context, method, path string, in RawIn, out any) error {
	ct := in.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	res, err := c.do(ctx, method, path, in.Query, in.Body, ct)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// CallRawOut calls an operation whose result is a body (a download); the caller closes it.
func (c *Caller) CallRawOut(ctx context.Context, method, path string, in any) (RawOut, error) {
	res, err := c.do(ctx, method, path, EncodeQuery(in), nil, "")
	if err != nil {
		return RawOut{}, err
	}
	return RawOut{Body: res.Body, ContentType: res.Header.Get("Content-Type")}, nil
}

// CallStream opens a GET streaming operation (CallStreamMethod with GET).
func (c *Caller) CallStream(ctx context.Context, path string, in any) (Stream, error) {
	return c.CallStreamMethod(ctx, http.MethodGet, path, in)
}

// CallStreamMethod opens a streaming operation: a GET sends in as the query, other methods as a JSON body. Its Initial
// is empty; Poll returns the entries received since the last call (decoded JSON); Close ends it; after Done, Err is
// the error the server ended it with, if any.
func (c *Caller) CallStreamMethod(ctx context.Context, method, path string, in any) (Stream, error) {
	ctx, cancel := context.WithCancel(ctx)
	var (
		res *http.Response
		err error
	)
	if method == http.MethodGet {
		res, err = c.do(ctx, method, path, EncodeQuery(in), nil, "")
	} else {
		b, merr := json.Marshal(in)
		if merr != nil {
			cancel()
			return Stream{}, merr
		}
		res, err = c.do(ctx, method, path, nil, bytes.NewReader(b), "application/json")
	}
	if err != nil {
		cancel()
		return Stream{}, err
	}
	var mu sync.Mutex
	var pending []any
	var serverErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		event := ""
		for sc.Scan() {
			line := sc.Text()
			if ev, ok := strings.CutPrefix(line, "event: "); ok {
				event = ev
				continue
			}
			data, ok := strings.CutPrefix(line, "data: ")
			if !ok {
				continue
			}
			if event == "error" {
				ce := &Error{}
				if json.Unmarshal([]byte(data), ce) != nil || ce.Code == "" {
					ce = &Error{Code: "internal", Message: data}
				}
				mu.Lock()
				serverErr = ce
				mu.Unlock()
				continue
			}
			var v any
			if json.Unmarshal([]byte(data), &v) == nil {
				mu.Lock()
				pending = append(pending, v)
				mu.Unlock()
			}
		}
	}()
	return Stream{
		Err: func() error {
			mu.Lock()
			defer mu.Unlock()
			return serverErr
		},
		Poll: func() []any {
			mu.Lock()
			defer mu.Unlock()
			out := pending
			pending = nil
			return out
		},
		Close: func() {
			cancel()
			_ = res.Body.Close()
			<-done
		},
		Done: done,
	}, nil
}

// ---------------------------------------------------------------------------------------------------------------
// MCP: a tool per JSON operation, calling the contract directly (not the HTTP route).

// Tool is one MCP tool of a wire contract.
type Tool struct {
	Name, Description string
	Contract          ContractID     // the contract it calls
	Scope             string         // the token scope it needs (a token limited to other scopes doesn't see it)
	ReadOnly          bool           // a GET: it only reads
	InputSchema       map[string]any // JSON Schema of its arguments, from the operation's input type
	Call              func(ctx context.Context, a Auth, args json.RawMessage) (any, error)
}

// Tools are the contract's MCP tools on app: <name>_<op>, e.g. deploy_check, remote_status. Uploads and downloads have
// no tool; a stream's tool returns its initial entries, and a pushed stream's (Run) everything it pushes until it ends.
func (w *Wire[T]) Tools(app core.App) []Tool {
	out := make([]Tool, 0, len(w.Ops))
	for _, op := range w.Ops {
		op := op
		if op.RawIn || op.Out == OutRaw {
			continue
		}
		scope := op.Scope
		if scope == "" {
			scope = w.Scope
		}
		out = append(out, Tool{
			Name:        ToolName(w.C.id, op.Name),
			Description: op.Desc,
			Contract:    w.C.id,
			Scope:       scope,
			ReadOnly:    op.method() == http.MethodGet,
			InputSchema: SchemaOf(op.In),
			Call: func(ctx context.Context, a Auth, args json.RawMessage) (any, error) {
				impl, ok := w.C.Of(app)
				if !ok {
					return nil, &Error{Code: "unavailable", Message: string(w.C.id) + " isn't provided on this app"}
				}
				a.Via = "mcp"
				req := OpRequest{JSON: args, Query: url.Values{}}
				if op.method() == http.MethodGet || op.method() == http.MethodDelete {
					var m map[string]any
					_ = json.Unmarshal(args, &m)
					for k, v := range m {
						req.Query.Set(k, fmt.Sprint(v))
					}
				}
				res, err := op.Invoke(ctx, impl, a, req)
				if s, isStream := res.(Stream); isStream && err == nil {
					if s.Close != nil {
						defer s.Close()
					}
					if s.Run == nil {
						return s.Initial, nil
					}
					entries := append([]any(nil), s.Initial...)
					err := s.Run(ctx, func(e any) error { entries = append(entries, e); return nil })
					return entries, err
				}
				return res, err
			},
		})
	}
	return out
}
