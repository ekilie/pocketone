package extkit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// JSPolicy says what an app's JS (pb_hooks) may do with a contract.
type JSPolicy int

const (
	// JSClosed: JS can't provide it (the default for a contract without a JS adapter).
	JSClosed JSPolicy = iota
	// JSHooks: JS can bind the contract's action hooks, never provide it.
	JSHooks
	// JSOpen: JS can provide it, and bind its hooks.
	JSOpen
)

// JSObject is a provider written in JS: an object whose methods run on the app's JS VM (they may use the file's
// top-level variables).
type JSObject struct {
	app     core.App
	id      ContractID
	methods map[string]js.Func
}

// Call runs method with args and returns its exported result (awaiting a returned promise). A thrown
// ContractError(code, message) comes back as *Error.
func (o *JSObject) Call(ctx context.Context, method string, args ...any) (any, error) {
	f := o.methods[method]
	if f == nil {
		return nil, &Error{Code: "not_implemented", Message: fmt.Sprintf("the pb_hooks provider of %s has no %s()", o.id, method)}
	}
	out, err := f.Call(args...)
	if err != nil {
		return nil, jsError(err)
	}
	return out, nil
}

// CallJSON calls method with args converted through JSON (so JS sees their json field names) and decodes the result
// into out (nil to ignore it).
func (o *JSObject) CallJSON(ctx context.Context, method string, out any, args ...any) error {
	plain := make([]any, len(args))
	for i, a := range args {
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &plain[i]); err != nil {
			return err
		}
	}
	res, err := o.Call(ctx, method, plain...)
	if err != nil || out == nil || res == nil {
		return err
	}
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return &Error{Code: "bad_result", Message: fmt.Sprintf("%s() in pb_hooks returned %s: %v", method, b, err), Status: 500}
	}
	return nil
}

// jsError turns a JS exception into the contract's typed error when it's a ContractError.
func jsError(err error) error {
	thrown, ok := js.Thrown(err)
	if !ok {
		return err
	}
	if m, isObj := thrown.(map[string]any); isObj {
		if code, has := m["code"]; has && code != nil {
			status := 400
			switch v := m["status"].(type) {
			case int64:
				status = int(v)
			case float64:
				status = int(v)
			}
			return &Error{Code: fmt.Sprint(code), Message: js.Message(err), Status: status}
		}
	}
	return &Error{Code: "js_error", Message: js.Message(err), Status: 500}
}

type jsAdapter struct {
	policy  JSPolicy
	provide func(app core.App, s *Scope, o *JSObject)
	host    bool   // host-providable: closed to JS when the host provides it
	types   string // the contract's TypeScript declarations (contractgen's zz_contract.d.ts), "" when it has none
}

var (
	jsMu       sync.Mutex
	jsAdapters = map[ContractID]jsAdapter{}
	jsWires    []func(app core.App, rt js.Runtime) // binds each wire contract's on<Contract><Op> functions
	jsHookOps  []jsHookOp                          // the same hooks, for TypeScript declarations
)

// JS registers the contract's JS adapter: how a JS object becomes a T, and what an app's JS may do with it.
func (c *Contract[T]) JS(policy JSPolicy, adapt func(o *JSObject) T) *Contract[T] {
	jsMu.Lock()
	defer jsMu.Unlock()
	jsAdapters[c.id] = jsAdapter{
		policy: policy,
		types:  jsAdapters[c.id].types,
		host:   c.hostProvidable,
		provide: func(app core.App, s *Scope, o *JSObject) {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.mustNotBeSealed("e.provide", c.id)
			sl := s.slot(c.id, c.card)
			sl.provisions = append(sl.provisions, provision{ext: "pb_hooks", value: adapt(o)})
		},
	}
	return c
}

// JSTypes sets the contract's TypeScript declarations for pb_hooks: its types, its action hooks typed per operation,
// and (for JSOpen) its entry in contracts.Providers, which types e.provide. contractgen generates them from the Go
// interface (zz_contract.d.ts). They reach the app's types.d.ts only while the contract is open to JS. Call it after JS.
func (c *Contract[T]) JSTypes(ts string) *Contract[T] {
	jsMu.Lock()
	defer jsMu.Unlock()
	a, ok := jsAdapters[c.id]
	if !ok {
		panic(fmt.Sprintf("extkit: %s: JSTypes before JS", c.id))
	}
	a.types = ts
	jsAdapters[c.id] = a
	return c
}

// JSContracts is the jsvm extension (a value for jsvm.Config.Extensions) that gives an app's pb_hooks `onContracts`
// (declare and provide contracts, see jsdeclare.go) and every wire contract's action hooks (`onDeployUpload(e =>
// ...)`). Register jsvm after the Go extensions and before the seal.
func JSContracts(app core.App) any { return jsContracts{app} }

type jsContracts struct {
	app core.App
}

// OnInit implements the jsvm extension method.
func (j jsContracts) OnInit(rt js.Runtime) {
	_, _ = rt.Eval(`globalThis.ContractError = class ContractError extends Error {
  constructor(code, message, status) { super(message); this.code = code; this.status = status }
}`, "<contracts>")
	j.bindOnContracts(rt) // onContracts(e => { e.provide(...); e.require(...); e.worksWith(...) })
	jsMu.Lock()
	binds := append([]func(core.App, js.Runtime){}, jsWires...)
	jsMu.Unlock()
	for _, b := range binds {
		b(j.app, rt)
	}
}

func (j jsContracts) provide(id ContractID, obj js.Object) error {
	jsMu.Lock()
	a, ok := jsAdapters[id]
	jsMu.Unlock()
	s := scopeOf(j.app)
	switch {
	case !ok:
		return fmt.Errorf("e.provide: %s has no JS adapter, so pb_hooks can't provide it", id)
	case a.policy != JSOpen:
		return fmt.Errorf("e.provide: %s is closed to pb_hooks (it can only be hooked)", id)
	case s.host != nil && s.host.HostProvides(id):
		return fmt.Errorf("e.provide: the host provides %s for this instance", id)
	}
	if s.sealed {
		return fmt.Errorf("e.provide(%s) after the seal", id)
	}
	o := &JSObject{app: j.app, id: id, methods: map[string]js.Func{}}
	for _, k := range obj.Keys() {
		if f := obj.Get(k).Func(); f != nil {
			o.methods[k] = f
		}
	}
	a.provide(j.app, s, o)
	return nil
}

type jsHookOp struct {
	id ContractID
	op string
}

// TypeScript implements the jsvm extension method (extkit/js Typer): onContracts, ContractError, and every contract
// pb_hooks may touch: its own declarations (JSTypes) when it has them, else its action hooks with an untyped input.
func (j jsContracts) TypeScript() string {
	var b strings.Builder
	b.WriteString(jsContractsTypes)
	jsMu.Lock()
	ops := append([]jsHookOp{}, jsHookOps...)
	open := map[ContractID]bool{}
	typed := map[ContractID]string{}
	for id, a := range jsAdapters {
		open[id] = a.policy != JSClosed
		if open[id] && a.types != "" {
			typed[id] = a.types
		}
	}
	jsMu.Unlock()
	ids := make([]string, 0, len(typed))
	for id := range typed {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		b.WriteString("\n")
		b.WriteString(typed[ContractID(id)])
	}
	seen := map[string]bool{}
	for _, o := range ops {
		name := jsHookName(o.id, o.op)
		if !open[o.id] || typed[o.id] != "" || seen[name] {
			continue
		}
		seen[name] = true
		fmt.Fprintf(&b, "/** Runs on every %s call to %s (change e.input, or throw to refuse). @group PocketOne */\n", o.op, o.id)
		fmt.Fprintf(&b, "declare function %s(handler: (e: contracts.CallEvent) => void): void;\n", name)
	}
	return b.String()
}

const jsContractsTypes = `// ---- contracts (ext/extkit): onContracts and the contracts' action hooks ----

declare namespace contracts {
  interface Auth {
    /** "superuser", "record", "token", "host", or "" (anonymous). */
    kind: string;
    id: string;
    /** A token's scopes; null means every scope. */
    scopes: string[] | null;
    via: string;
    collection: string;
    recordId: string;
    email: string;
  }
  /** One call to a wire contract's operation; In is the operation's input type (its contract's namespace has it). */
  interface CallEvent<In = any> {
    contract: string;
    op: string;
    auth: Auth;
    /** The operation's input; a hook may change it. */
    input: In;
    next(): void;
  }
  /**
   * What e.provide takes for each contract pb_hooks can provide, by id ("mail.fallback/v1": its interface). Each
   * contract's declarations add their entry.
   */
  interface Providers {}
  /** A JS implementation of a contract with no declarations here: an object with its methods. */
  interface AnyProvider {
    [method: string]: (...args: any[]) => any;
  }
  interface DeclareEvent {
    /** Provides a contract from JS: an object with its methods (typed by contracts.Providers for known ids). */
    provide<Id extends string>(id: Id, impl: Id extends keyof Providers ? Providers[Id] : AnyProvider): void;
    /** The app needs this contract: the start fails (dev mode: warns) when nothing provides it. */
    require(...ids: string[]): void;
    /** Works with this contract when present (shown in the startup lines). */
    worksWith(...ids: string[]): void;
    next(): void;
  }
}

/**
 * Declares and provides the app's contracts, once, after pb_hooks load.
 *
 * @group PocketOne
 */
declare function onContracts(handler: (e: contracts.DeclareEvent) => void): void;

/** An error a contract implementation throws: a code, a message and an HTTP status. @group PocketOne */
declare class ContractError extends Error {
  constructor(code: string, message: string, status?: number);
  code: string;
  status?: number;
}
`

// jsHookName is on<Contract><Op>: deploy/v1 + upload → onDeployUpload; mail.fallback/v1 + send → onMailFallbackSend.
func jsHookName(id ContractID, op string) string {
	base := strings.SplitN(string(id), "/", 2)[0]
	var b strings.Builder
	b.WriteString("on")
	for _, part := range strings.FieldsFunc(base+"."+op, func(r rune) bool { return r == '.' || r == '_' || r == '-' }) {
		r := []rune(part)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

// bindJSHooks makes a wire contract's action hooks bindable from pb_hooks, under JSHooks and JSOpen policies.
func bindJSHooks(id ContractID, ops []string, on func(owner any, op string) *hook.Hook[*CallEvent]) {
	jsMu.Lock()
	defer jsMu.Unlock()
	for _, op := range ops {
		jsHookOps = append(jsHookOps, jsHookOp{id, op})
	}
	jsWires = append(jsWires, func(app core.App, rt js.Runtime) {
		jsMu.Lock()
		a, ok := jsAdapters[id]
		jsMu.Unlock()
		if !ok || a.policy == JSClosed {
			return
		}
		for _, op := range ops {
			op := op
			name := jsHookName(id, op)
			_ = rt.Set(name, js.NativeFunc(func(c js.Call) (any, error) {
				if scopeOf(app).sealed {
					return nil, fmt.Errorf("%s after the seal: call it at the top level of a .pb.js file", name)
				}
				f := c.Arg(0).Func()
				if f == nil {
					return nil, fmt.Errorf("%s(handler): handler must be a function", name)
				}
				on(app, op).BindFunc(func(e *CallEvent) error {
					res, err := f.Call(e)
					if err != nil {
						return jsError(err)
					}
					if rerr, isErr := res.(error); isErr {
						return rerr
					}
					return nil
				})
				return nil, nil
			}))
		}
	})
}
