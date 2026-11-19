package apitokens

import (
	"errors"
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// BindJS exposes scopes to an app's JS hooks as the $apiTokens global. Add it to the jsvm OnInit (the standard
// binary does):
//
//	jsvm.MustRegister(app, jsvm.Config{OnInit: func(rt js.Runtime) { apitokens.BindJS(app, rt) }})
//
// In a pb_hooks file:
//
//	// at the top level: shows up in Settings → API tokens and on the OAuth consent page
//	$apiTokens.registerScope({ name: "notes:read", description: "Read notes", paths: ["/api/collections/notes/records"], methods: ["GET"] })
//	$apiTokens.registerScope({ name: "notes:write", description: "Change notes", paths: ["/api/collections/notes/records"] })
//
//	// guard a route: signed in, and a token (if any) holding the scope
//	routerAdd("POST", "/api/notes/archive", (e) => { ... }, $apis.requireAuth(), $apiTokens.requireScope("notes:write"))
//
//	$apiTokens.hasScope(e, "notes:write")   // inside a handler
//	$apiTokens.ruleField("notes:write")     // "@request.headers.x_token_scope_notes_write", for collection rules
//	$apiTokens.scopes()                     // every registered scope
//
// Registering the same name again (e.g. when hooks reload) merges it.
func BindJS(app core.App, rt js.Runtime) {
	get := func() (*Tokens, error) {
		t := For(app)
		if t == nil {
			return nil, errors.New("api-tokens isn't registered on this app")
		}
		return t, nil
	}
	_ = rt.Set("$apiTokens", map[string]any{
		"registerScope": func(def map[string]any) error {
			s := Scope{Name: str(def["name"]), Description: str(def["description"])}
			var err error
			for field, dst := range map[string]*[]string{"paths": &s.Paths, "methods": &s.Methods, "collections": &s.Collections} {
				if *dst, err = strs(def[field]); err != nil {
					return errors.New("$apiTokens.registerScope: " + field + " must be a list of strings")
				}
			}
			register := func(s Scope) error { return pendingFor(app).add(s) } // pb_hooks loaded before apitokens.Register
			if t := For(app); t != nil {
				register = t.RegisterScope
			}
			if err := register(s); err != nil {
				return errors.New("$apiTokens.registerScope: " + err.Error())
			}
			return nil
		},
		"requireScope": func(name string) any { return RequireScope(name) },
		"hasScope":     func(e *core.RequestEvent, name string) bool { return HasScope(e, name) },
		"ruleField":    func(name string) string { return ScopeRuleField(name) },
		"scopes": func() ([]map[string]any, error) {
			t, err := get()
			if err != nil {
				return nil, err
			}
			out := []map[string]any{}
			for _, s := range t.Scopes() {
				out = append(out, map[string]any{
					"name": s.Name, "description": s.Description, "paths": s.Paths, "methods": s.Methods,
					"collections": s.Collections, "header": s.Header(), "ruleField": s.RuleField(),
				})
			}
			return out, nil
		},
	})
}

const pendingKey = "api-tokens.pending"

// pendingFor holds the scopes pb_hooks registered before apitokens.Register ran (the stock binary loads the jsvm
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
func (p *Extension) BindJS(app core.App, rt js.Runtime) { BindJS(app, rt) }

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func strs(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, errors.New("not a list")
	}
	out := make([]string, len(items))
	for i, x := range items {
		s, ok := x.(string)
		if !ok {
			return nil, errors.New("not a string")
		}
		out[i] = s
	}
	return out, nil
}
