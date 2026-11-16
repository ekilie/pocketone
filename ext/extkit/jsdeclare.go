package extkit

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/dop251/goja"
)

// An app's JS declares and provides its contracts in onContracts, the way a Go extension does in Contracts() and
// Register:
//
//	onContracts((e) => {
//	  e.provide("mail.fallback/v1", { send(m) { ... } })
//	  e.require("restore/v1")   // the app needs it: the start fails (dev mode: warns) when nothing provides it
//	  e.worksWith("tokens/v1")  // shown in the startup lines
//	  e.next()
//	})
//
// The handlers run once, after pb_hooks load and before the seal (so before the app bootstraps): every bootstrap
// handler, Go or JS, sees the final wiring. They run on a pooled runtime, so like any hook handler they can't use the
// file's local variables.

const jsDeclKey = "extkit.jsdecl"

type jsDecl struct {
	mu        sync.Mutex
	run       JSRunner
	handlers  []*goja.Program
	ran       bool
	requires  []ContractID
	worksWith []ContractID
	provides  []ContractID
}

func jsDeclOf(app core.App) *jsDecl {
	return app.Store().GetOrSet(jsDeclKey, func() any { return &jsDecl{} }).(*jsDecl)
}

// bindOnContracts adds the onContracts global to a runtime. Only the loader runs hook files, so a handler registers
// once per load.
func (j jsContracts) bindOnContracts(vm *goja.Runtime) {
	d := jsDeclOf(j.app)
	d.mu.Lock()
	d.run = j.run
	d.mu.Unlock()
	vm.Set("onContracts", func(handler goja.Value) {
		if _, ok := goja.AssertFunction(handler); !ok {
			panic(vm.NewTypeError("onContracts(handler): handler must be a function"))
		}
		if scopeOf(j.app).sealed {
			panic(vm.NewGoError(fmt.Errorf("onContracts after the seal: call it at the top level of a .pb.js file")))
		}
		pr, err := goja.Compile("onContracts", "("+handler.String()+").apply(undefined, __args)", true)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		d.mu.Lock()
		d.handlers = append(d.handlers, pr)
		d.mu.Unlock()
	})
}

// runJSDeclarations runs the app's onContracts handlers, once, before its contracts seal.
func runJSDeclarations(app core.App) error {
	v, ok := app.Store().Get(jsDeclKey).(*jsDecl)
	if !ok {
		return nil
	}
	v.mu.Lock()
	if v.ran || len(v.handlers) == 0 {
		v.ran = true
		v.mu.Unlock()
		return nil
	}
	v.ran = true
	handlers, run := append([]*goja.Program(nil), v.handlers...), v.run
	v.mu.Unlock()
	j := jsContracts{app: app, run: run}
	for _, pr := range handlers {
		err := run(app, func(vm *goja.Runtime) error {
			e := vm.NewObject()
			_ = e.Set("provide", func(call goja.FunctionCall) goja.Value {
				id := ContractID(call.Argument(0).String())
				obj, isObj := call.Argument(1).(*goja.Object)
				if !isObj {
					panic(vm.NewTypeError("e.provide(%q, object): the second argument must be an object", id))
				}
				if err := j.provide(vm, id, obj); err != nil {
					panic(vm.NewGoError(err))
				}
				v.mu.Lock()
				v.provides = append(v.provides, id)
				v.mu.Unlock()
				return goja.Undefined()
			})
			list := func(dst *[]ContractID) func(goja.FunctionCall) goja.Value {
				return func(call goja.FunctionCall) goja.Value {
					v.mu.Lock()
					for _, a := range call.Arguments {
						*dst = append(*dst, ContractID(a.String()))
					}
					v.mu.Unlock()
					return goja.Undefined()
				}
			}
			_ = e.Set("require", list(&v.requires))
			_ = e.Set("worksWith", list(&v.worksWith))
			_ = e.Set("next", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
			vm.Set("__args", []any{e})
			defer vm.Set("__args", goja.Undefined())
			if _, err := vm.RunProgram(pr); err != nil {
				return jsError(vm, err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("onContracts in pb_hooks: %w", err)
		}
	}
	return nil
}

// checkJSDeclarations runs after the seal: every contract the app's JS requires has a provider (else an error, or a
// warning in dev mode), and the startup line says what the JS provides, requires and works with.
func checkJSDeclarations(app core.App, s *Scope) error {
	v, ok := app.Store().Get(jsDeclKey).(*jsDecl)
	if !ok {
		return nil
	}
	v.mu.Lock()
	requires, worksWith, provides := uniq(v.requires), uniq(v.worksWith), uniq(v.provides)
	v.mu.Unlock()
	var missing []string
	for _, id := range requires {
		if !s.has(id) {
			missing = append(missing, string(id))
		}
	}
	var parts []string
	for _, p := range []struct {
		label string
		ids   []ContractID
	}{{"provides", provides}, {"requires", requires}, {"works with", worksWith}} {
		if len(p.ids) > 0 {
			ids := make([]string, len(p.ids))
			for i, id := range p.ids {
				ids[i] = string(id)
			}
			parts = append(parts, p.label+" "+strings.Join(ids, ", "))
		}
	}
	if len(parts) > 0 {
		s.mu.Lock()
		s.Lines = append(s.Lines, "pb_hooks: "+strings.Join(parts, "; "))
		s.mu.Unlock()
	}
	if len(missing) == 0 {
		return nil
	}
	err := fmt.Errorf("extkit: pb_hooks requires %s, which nothing provides", strings.Join(missing, ", "))
	if app.IsDev() {
		app.Logger().Warn(err.Error())
		return nil
	}
	return err
}

// has reports whether the sealed scope has a provider for id (its own, or its host's).
func (s *Scope) has(id ContractID) bool {
	if s.host != nil && s.host.HostProvides(id) {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl := s.slots[id]
	return sl != nil && len(sl.sealed) > 0
}

func uniq(ids []ContractID) []ContractID {
	seen := map[ContractID]bool{}
	var out []ContractID
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
