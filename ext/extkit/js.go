package extkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
	"github.com/dop251/goja"
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

// JSObject is a provider written in JS: an object whose methods run in the app's pooled JS runtimes, like hooks.
type JSObject struct {
	app      core.App
	run      JSRunner
	id       ContractID
	programs map[string]*goja.Program
}

// Call runs method with args on one of the app's pooled runtimes and returns its exported result. A thrown
// ContractError(code, message) comes back as *Error.
func (o *JSObject) Call(ctx context.Context, method string, args ...any) (any, error) {
	pr := o.programs[method]
	if pr == nil {
		return nil, &Error{Code: "not_implemented", Message: fmt.Sprintf("the pb_hooks provider of %s has no %s()", o.id, method)}
	}
	var out any
	err := o.run(o.app, func(vm *goja.Runtime) error {
		vm.Set("__args", args)
		defer vm.Set("__args", goja.Undefined())
		res, err := vm.RunProgram(pr)
		if err != nil {
			return jsError(vm, err)
		}
		if res != nil && !goja.IsUndefined(res) && !goja.IsNull(res) {
			out = res.Export()
		}
		return nil
	})
	return out, err
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
func jsError(vm *goja.Runtime, err error) error {
	ex, ok := err.(*goja.Exception)
	if !ok {
		return err
	}
	if obj, isObj := ex.Value().(*goja.Object); isObj {
		if code := obj.Get("code"); code != nil && !goja.IsUndefined(code) {
			msg := ""
			if m := obj.Get("message"); m != nil {
				msg = m.String()
			}
			status := 400
			if s := obj.Get("status"); s != nil && !goja.IsUndefined(s) {
				status = int(s.ToInteger())
			}
			return &Error{Code: code.String(), Message: msg, Status: status}
		}
	}
	return &Error{Code: "js_error", Message: ex.Value().String(), Status: 500}
}

type jsAdapter struct {
	policy  JSPolicy
	provide func(app core.App, s *Scope, o *JSObject)
	host    bool // host-providable: closed to JS when the host provides it
}

var (
	jsMu       sync.Mutex
	jsAdapters = map[ContractID]jsAdapter{}
	jsWires    []func(app core.App, run JSRunner, vm *goja.Runtime) // binds each wire contract's on<Contract><Op> functions
)

// JS registers the contract's JS adapter: how a JS object becomes a T, and what an app's JS may do with it.
func (c *Contract[T]) JS(policy JSPolicy, adapt func(o *JSObject) T) *Contract[T] {
	jsMu.Lock()
	defer jsMu.Unlock()
	jsAdapters[c.id] = jsAdapter{
		policy: policy,
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

// JSRunner runs fn on one of app's pooled JS runtimes: jsvm.Run. Passed in so extkit doesn't import jsvm (and its
// dependencies) into every extension.
type JSRunner func(app core.App, fn func(vm *goja.Runtime) error) error

// JSContracts is the jsvm extension (a value for jsvm.Config.Extensions) that gives an app's pb_hooks `onContracts`
// (declare and provide contracts, see jsdeclare.go) and every wire contract's action hooks (`onDeployUpload(e =>
// ...)`). run is jsvm.Run. Register jsvm after the Go extensions and before the seal.
func JSContracts(app core.App, run JSRunner) any { return jsContracts{app, run} }

type jsContracts struct {
	app core.App
	run JSRunner
}

func (j jsContracts) OnInit(vm *goja.Runtime) {
	_, _ = vm.RunString(`globalThis.ContractError = class ContractError extends Error {
  constructor(code, message, status) { super(message); this.code = code; this.status = status }
}`)
	j.bindOnContracts(vm) // onContracts(e => { e.provide(...); e.require(...); e.worksWith(...) })
	jsMu.Lock()
	binds := append([]func(core.App, JSRunner, *goja.Runtime){}, jsWires...)
	jsMu.Unlock()
	for _, b := range binds {
		b(j.app, j.run, vm)
	}
}

func (j jsContracts) provide(vm *goja.Runtime, id ContractID, obj *goja.Object) error {
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
	o := &JSObject{app: j.app, run: j.run, id: id, programs: map[string]*goja.Program{}}
	for _, k := range obj.Keys() {
		fn, isFn := goja.AssertFunction(obj.Get(k))
		if !isFn || fn == nil {
			continue
		}
		pr, err := compileMethod(k, obj.Get(k).String())
		if err != nil {
			return fmt.Errorf("e.provide(%s): %s(): %w", id, k, err)
		}
		o.programs[k] = pr
	}
	a.provide(j.app, s, o)
	return nil
}

// compileMethod compiles a method's source so it runs on any pooled runtime, the way jsvm re-compiles hook handlers:
// it can't capture the file's local variables. Shorthand methods (`send(m) {...}`) and function values both work.
func compileMethod(name, src string) (*goja.Program, error) {
	shorthand := fmt.Sprintf("({ %s })[%q].apply(undefined, __args)", src, name)
	if pr, err := goja.Compile(name, shorthand, true); err == nil {
		return pr, nil
	}
	return goja.Compile(name, fmt.Sprintf("(%s).apply(undefined, __args)", src), true)
}

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
	jsWires = append(jsWires, func(app core.App, run JSRunner, vm *goja.Runtime) {
		jsMu.Lock()
		a, ok := jsAdapters[id]
		jsMu.Unlock()
		if !ok || a.policy == JSClosed {
			return
		}
		for _, op := range ops {
			op := op
			vm.Set(jsHookName(id, op), func(handler goja.Value) {
				if scopeOf(app).sealed {
					return // executors run every file's top level too; only the loader registers
				}
				pr, err := goja.Compile(jsHookName(id, op), "("+handler.String()+").apply(undefined, __args)", true)
				if err != nil {
					panic(vm.NewGoError(err))
				}
				on(app, op).BindFunc(func(e *CallEvent) error {
					return run(app, func(vm *goja.Runtime) error {
						vm.Set("__args", []any{e})
						defer vm.Set("__args", goja.Undefined())
						if _, err := vm.RunProgram(pr); err != nil {
							return jsError(vm, err)
						}
						return nil
					})
				})
			})
		}
	})
}
