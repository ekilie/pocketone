package extkit

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit/js"
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
// handler, Go or JS, sees the final wiring. Like every handler on ext/jsvm, they can use the file's top-level code.

const jsDeclKey = "extkit.jsdecl"

type jsDecl struct {
	mu        sync.Mutex
	handlers  []js.Func
	ran       bool
	requires  []ContractID
	worksWith []ContractID
	provides  []ContractID
}

func jsDeclOf(app core.App) *jsDecl {
	return app.Store().GetOrSet(jsDeclKey, func() any { return &jsDecl{} }).(*jsDecl)
}

// bindOnContracts adds the onContracts global.
func (j jsContracts) bindOnContracts(rt js.Runtime) {
	d := jsDeclOf(j.app)
	_ = rt.Set("onContracts", js.NativeFunc(func(c js.Call) (any, error) {
		f := c.Arg(0).Func()
		if f == nil {
			return nil, fmt.Errorf("onContracts(handler): handler must be a function")
		}
		if scopeOf(j.app).sealed {
			return nil, fmt.Errorf("onContracts after the seal: call it at the top level of a .pb.js file")
		}
		d.mu.Lock()
		d.handlers = append(d.handlers, f)
		d.mu.Unlock()
		return nil, nil
	}))
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
	handlers := append([]js.Func(nil), v.handlers...)
	v.mu.Unlock()
	j := jsContracts{app: app}
	list := func(dst *[]ContractID) js.NativeFunc {
		return func(c js.Call) (any, error) {
			v.mu.Lock()
			for i := 0; i < c.Len(); i++ {
				*dst = append(*dst, ContractID(c.Arg(i).String()))
			}
			v.mu.Unlock()
			return nil, nil
		}
	}
	e := map[string]any{
		"provide": js.NativeFunc(func(c js.Call) (any, error) {
			id := ContractID(c.Arg(0).String())
			obj := c.Arg(1).Object()
			if obj == nil {
				return nil, fmt.Errorf("e.provide(%q, object): the second argument must be an object", id)
			}
			if err := j.provide(id, obj); err != nil {
				return nil, err
			}
			v.mu.Lock()
			v.provides = append(v.provides, id)
			v.mu.Unlock()
			return nil, nil
		}),
		"require":   list(&v.requires),
		"worksWith": list(&v.worksWith),
		"next":      js.NativeFunc(func(js.Call) (any, error) { return nil, nil }),
	}
	for _, f := range handlers {
		if _, err := f.Call(e); err != nil {
			return fmt.Errorf("onContracts in pb_hooks: %w", jsError(err))
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
