package tokens

import (
	"errors"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/dop251/goja"
)

// BindJS exposes scopes to an app's JS hooks as the $tokens global. Add it to the jsvm OnInit (the standard
// binary does):
//
//	jsvm.MustRegister(app, jsvm.Config{OnInit: func(vm *goja.Runtime) { tokens.BindJS(vm, app) }})
//
// In a pb_hooks file:
//
//	// at the top level: shows up in Settings → API tokens and on the OAuth consent page
//	$tokens.registerScope({ name: "notes:read", description: "Read notes", paths: ["/api/collections/notes/records"], methods: ["GET"] })
//	$tokens.registerScope({ name: "notes:write", description: "Change notes", paths: ["/api/collections/notes/records"] })
//
//	// guard a route: signed in, and a token (if any) holding the scope
//	routerAdd("POST", "/api/notes/archive", (e) => { ... }, $apis.requireAuth(), $tokens.requireScope("notes:write"))
//
//	$tokens.hasScope(e, "notes:write")   // inside a handler
//	$tokens.ruleField("notes:write")     // "@request.headers.x_token_scope_notes_write", for collection rules
//	$tokens.scopes()                     // every registered scope
//
// Registering the same name again (e.g. when hooks reload) merges it.
func BindJS(vm *goja.Runtime, app core.App) {
	get := func() *Tokens {
		t := For(app)
		if t == nil {
			panic(vm.NewGoError(errors.New("tokens isn't registered on this app")))
		}
		return t
	}
	obj := vm.NewObject()

	_ = obj.Set("registerScope", func(call goja.FunctionCall) goja.Value {
		def := call.Argument(0).ToObject(vm)
		s := Scope{
			Name:        jsStr(def.Get("name")),
			Description: jsStr(def.Get("description")),
			Paths:       jsStrings(vm, def.Get("paths"), "paths"),
			Methods:     jsStrings(vm, def.Get("methods"), "methods"),
			Collections: jsStrings(vm, def.Get("collections"), "collections"),
		}
		register := func(s Scope) error { return pendingFor(app).add(s) } // pb_hooks loaded before tokens.Register
		if t := For(app); t != nil {
			register = t.RegisterScope
		}
		if err := register(s); err != nil {
			panic(vm.NewTypeError("$tokens.registerScope: " + err.Error()))
		}
		return goja.Undefined()
	})

	_ = obj.Set("requireScope", func(name string) any { return RequireScope(name) })

	_ = obj.Set("hasScope", func(e *core.RequestEvent, name string) bool { return HasScope(e, name) })

	_ = obj.Set("ruleField", func(name string) string { return ScopeRuleField(name) })

	_ = obj.Set("scopes", func() []map[string]any {
		out := []map[string]any{}
		for _, s := range get().Scopes() {
			out = append(out, map[string]any{
				"name": s.Name, "description": s.Description, "paths": s.Paths, "methods": s.Methods,
				"collections": s.Collections, "header": s.Header(), "ruleField": s.RuleField(),
			})
		}
		return out
	})

	_ = vm.Set("$tokens", obj)
}

const pendingKey = "tokens.pending"

// pendingFor holds the scopes pb_hooks registered before tokens.Register ran (the stock binary loads the jsvm
// first); Register takes them over.
func pendingFor(app core.App) *registry {
	var r *registry
	app.Store().SetFunc(pendingKey, func(old any) any {
		if p, ok := old.(*registry); ok {
			r = p
			return p
		}
		r = &registry{}
		return r
	})
	return r
}

// BindJS implements extkit.JSBinder.
func (p *Extension) BindJS(app core.App, vm *goja.Runtime) { BindJS(vm, app) }

func jsStr(v goja.Value) string {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return ""
	}
	return v.String()
}

func jsStrings(vm *goja.Runtime, v goja.Value, field string) []string {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	var out []string
	if err := vm.ExportTo(v, &out); err != nil {
		panic(vm.NewTypeError("$tokens.registerScope: " + field + " must be a list of strings"))
	}
	return out
}
