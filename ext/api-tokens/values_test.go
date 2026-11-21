package apitokens

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/tools/router"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// newInstancesApp: "instances" takes values (owned ids are "own1"/"own2"); GET /api/inst/{id} answers only when the
// request holds the scope for that id; /api/inst/echo shows the scope header.
func newInstancesApp(t *testing.T) (*tests.TestApp, *Tokens) {
	t.Helper()
	app, tk := newApp(t, Config{})
	tk.MustRegisterScope(Scope{Name: "instances", Description: "Your instances", Paths: []string{"/api/inst/"},
		Values: &ScopeValues{Description: "an instance id", Check: func(owner *core.Record, v string) error {
			if v != "own1" && v != "own2" && v != "Own3" {
				return errors.New("not your instance")
			}
			return nil
		}}})
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/inst/echo", func(e *core.RequestEvent) error {
			vals, all := ValuesFor(e, "instances")
			return e.JSON(http.StatusOK, map[string]any{"header": e.Request.Header.Get("X-Token-Scope-Instances"), "values": vals, "all": all})
		})
		se.Router.GET("/api/inst/{id}", func(e *core.RequestEvent) error {
			if !HasScopeFor(e, "instances", e.Request.PathValue("id")) {
				return e.ForbiddenError("not this instance", nil)
			}
			return e.String(http.StatusOK, "ok "+e.Request.PathValue("id"))
		})
		return se.Next()
	})
	return app, tk
}

func TestScopeValuesCreate(t *testing.T) {
	app, tk := newInstancesApp(t)
	su := superuser(t, app)

	_, rec, err := tk.Create(su, CreateOptions{Scopes: []string{"instances=own2", "INSTANCES=own1", "instances=Own3", "deploy"}})
	if err != nil {
		t.Fatal(err)
	}
	// names lower-cased, values kept as given, sorted
	if got := strings.Join(scopesOf(rec), ","); got != "deploy,instances=Own3,instances=own1,instances=own2" {
		t.Fatal(got)
	}
	// a bare scope already holds every value
	_, rec, _ = tk.Create(su, CreateOptions{Scopes: []string{"instances=own1", "instances"}})
	if got := strings.Join(scopesOf(rec), ","); got != "instances" {
		t.Fatal(got)
	}
	for scope, want := range map[string]string{
		"deploy=x":        "takes no value",
		"instances=other": "not your instance",
		"instances=":      "invalid value",
		"instances=a b":   "invalid value",
		"nope=x":          "unknown scope",
	} {
		if _, _, err := tk.Create(su, CreateOptions{Scopes: []string{scope}}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", scope, err)
		}
	}
}

func TestScopeValuesRequests(t *testing.T) {
	app, tk := newInstancesApp(t)
	tk.MustRegisterScope(Scope{Name: "tokens", Paths: []string{"/api/_c/api-tokens/v1"}}) // lets scoped tokens mint
	su := superuser(t, app)
	one, _, _ := tk.Create(su, CreateOptions{Scopes: []string{"instances=own1", "tokens"}})
	two, _, _ := tk.Create(su, CreateOptions{Scopes: []string{"instances=own1", "instances=own2", "tokens"}})
	all, _, _ := tk.Create(su, CreateOptions{Scopes: []string{"instances"}})
	full, _, _ := tk.Create(su, CreateOptions{})

	factory := func(testing.TB) *tests.TestApp { return app }
	scenarios := []tests.ApiScenario{
		{Name: "its value", Method: http.MethodGet, URL: "/api/inst/own1", Headers: authHeader(one),
			ExpectedStatus: 200, ExpectedContent: []string{"ok own1"}},
		{Name: "another value", Method: http.MethodGet, URL: "/api/inst/own2", Headers: authHeader(one),
			ExpectedStatus: 403, ExpectedContent: []string{"Not this instance"}},
		{Name: "a valued scope still keeps to its paths", Method: http.MethodGet, URL: "/api/collections", Headers: authHeader(one),
			ExpectedStatus: 403, ExpectedContent: []string{"scopes"}},
		{Name: "the bare scope: any value", Method: http.MethodGet, URL: "/api/inst/own2", Headers: authHeader(all),
			ExpectedStatus: 200, ExpectedContent: []string{"ok own2"}},
		{Name: "header lists the values", Method: http.MethodGet, URL: "/api/inst/echo", Headers: authHeader(two),
			ExpectedStatus: 200, ExpectedContent: []string{`"header":"own1,own2"`, `"all":false`, `"values":["own1","own2"]`}},
		{Name: "header is 1 for every value", Method: http.MethodGet, URL: "/api/inst/echo", Headers: authHeader(all),
			ExpectedStatus: 200, ExpectedContent: []string{`"header":"1"`, `"all":true`}},
		{Name: "unscoped token: every value", Method: http.MethodGet, URL: "/api/inst/echo", Headers: authHeader(full),
			ExpectedStatus: 200, ExpectedContent: []string{`"header":"1"`, `"all":true`}},
		{Name: "a client can't fake the header", Method: http.MethodGet, URL: "/api/inst/echo",
			Headers:        map[string]string{"Authorization": "Bearer " + one, "X-Token-Scope-Instances": "1"},
			ExpectedStatus: 200, ExpectedContent: []string{`"header":"own1"`}},
		// a scoped token mints only within its own values
		{Name: "mint a narrower token", Method: http.MethodPost, URL: "/api/_c/api-tokens/v1", Headers: authHeader(two),
			Body: strings.NewReader(`{"name":"n","scopes":["instances=own2"]}`), ExpectedStatus: 200, ExpectedContent: []string{`"instances=own2"`}},
		{Name: "can't mint another value", Method: http.MethodPost, URL: "/api/_c/api-tokens/v1", Headers: authHeader(one),
			Body: strings.NewReader(`{"name":"n","scopes":["instances=own2"]}`), ExpectedStatus: 403, ExpectedContent: []string{"more scopes"}},
		{Name: "can't mint every value", Method: http.MethodPost, URL: "/api/_c/api-tokens/v1", Headers: authHeader(one),
			Body: strings.NewReader(`{"name":"n","scopes":["instances"]}`), ExpectedStatus: 403, ExpectedContent: []string{"more scopes"}},
		{Name: "scopes list says it takes values", Method: http.MethodGet, URL: "/api/_c/api-tokens/v1/scopes", Headers: authHeader(full),
			ExpectedStatus: 200, ExpectedContent: []string{`"values":{"description":"an instance id"}`}},
	}
	for _, s := range scenarios {
		s.TestAppFactory = factory
		s.DisableTestAppCleanup = true
		s.Test(t)
	}
}

func TestScopeValuesWithScopes(t *testing.T) {
	app, tk := newInstancesApp(t)
	su := superuser(t, app)
	all, _, _ := tk.Create(su, CreateOptions{Scopes: []string{"instances"}})
	jwt, _ := su.NewAuthToken()
	var r *router.Router[*core.RequestEvent]
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		r = se.Router
		// /api/replay?scopes=a,b&path=/x replays a GET in process, limited to scopes
		se.Router.GET("/api/inst/replay", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			ctx := WithScopes(e.Request.Context(), strings.Split(q.Get("scopes"), ","))
			mux, err := r.BuildMux()
			if err != nil {
				return err
			}
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, q.Get("path"), nil)
			req.Header.Set("Authorization", e.Request.Header.Get("Authorization"))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			return e.String(http.StatusOK, fmt.Sprintf("%d %s", w.Code, w.Body.String()))
		})
		return se.Next()
	})
	replay := func(scopes, path string) string {
		return "/api/inst/replay?scopes=" + url.QueryEscape(scopes) + "&path=" + path
	}
	factory := func(testing.TB) *tests.TestApp { return app }
	for _, s := range []tests.ApiScenario{
		{Name: "JWT limited to one value", Method: http.MethodGet, URL: replay("instances=own1", "/api/inst/own1"),
			Headers: authHeader(jwt), ExpectedStatus: 200, ExpectedContent: []string{"200 ok own1"}},
		{Name: "JWT limited: another value", Method: http.MethodGet, URL: replay("instances=own1", "/api/inst/own2"),
			Headers: authHeader(jwt), ExpectedStatus: 200, ExpectedContent: []string{"403 "}},
		{Name: "bare token narrowed to a value", Method: http.MethodGet, URL: replay("instances=own2", "/api/inst/echo"),
			Headers: authHeader(all), ExpectedStatus: 200, ExpectedContent: []string{"200 ", `"header":"own2"`, `"values":["own2"]`}},
		{Name: "bare token narrowed: another value", Method: http.MethodGet, URL: replay("instances=own2", "/api/inst/own1"),
			Headers: authHeader(all), ExpectedStatus: 200, ExpectedContent: []string{"403 "}},
	} {
		s.TestAppFactory = factory
		s.DisableTestAppCleanup = true
		s.Test(t)
	}
}

func TestScopeValuesJS(t *testing.T) {
	app, tk := newApp(t, Config{})
	vm := jsVM(t, app)
	if _, err := vm.RunString(`$apiTokens.registerScope({ name: "notes", paths: ["/api/notes/"], values: { description: "a note id" } })`); err != nil {
		t.Fatal(err)
	}
	s, _ := tk.registry.get("notes")
	if s.Values == nil || s.Values.Description != "a note id" {
		t.Fatalf("%+v", s)
	}
	v, err := vm.RunString(`$apiTokens.scopes().filter((s) => s.name == "notes")[0].values.description`)
	if err != nil || v.Export() != "a note id" {
		t.Fatal(v, err)
	}
}

func TestScopeValuesOAuthRequest(t *testing.T) {
	app, tk := newInstancesApp(t)
	_ = tk
	srv := &OAuthServer{app: app}
	got, err := srv.resolveScopes("instances=own1 deploy", nil)
	if err != nil || len(got) != 2 || got[0].Name != "instances=own1" || !strings.Contains(got[0].Description, "only own1") {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := srv.resolveScopes("deploy=x", nil); err == nil {
		t.Fatal("value on a scope without values")
	}
}
