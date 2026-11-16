package tokens

import (
	"fmt"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/router"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"
	"github.com/dop251/goja"
)

func TestScopeNamesAndHeaders(t *testing.T) {
	_, tk := newApp(t, Config{})
	for _, bad := range []string{"", "Notes", "notes write", "notes-write", ":x", "notes.write", strings.Repeat("a", 65)} {
		if err := tk.RegisterScope(Scope{Name: bad}); err == nil {
			t.Errorf("%q: registered", bad)
		}
	}
	if err := tk.RegisterScope(Scope{Name: "notes:write", Paths: []string{"/a"}}); err != nil {
		t.Fatal(err)
	}
	if err := tk.RegisterScope(Scope{Name: "notes_write"}); err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("collision: %v", err)
	}
	// the same name again merges (hooks reload, or two extensions sharing a scope)
	if err := tk.RegisterScope(Scope{Name: "notes:write", Paths: []string{"/a", "/b"}, Methods: []string{"post"}}); err != nil {
		t.Fatal(err)
	}
	s, _ := tk.registry.get("notes:write")
	if strings.Join(s.Paths, ",") != "/a,/b" || strings.Join(s.Methods, ",") != "POST" {
		t.Fatalf("merged: %+v", s)
	}
	if s.Header() != "X-Token-Scope-Notes-Write" || s.RuleField() != "@request.headers.x_token_scope_notes_write" {
		t.Fatalf("header %q rule %q", s.Header(), s.RuleField())
	}
	if ScopeRuleField("mcp:read") != "@request.headers.x_token_scope_mcp_read" {
		t.Fatal(ScopeRuleField("mcp:read"))
	}
}

// notes: list needs notes:read, create needs notes:write, both through the scope headers in the rules.
func newNotesApp(t *testing.T) (*tests.TestApp, *Tokens) {
	t.Helper()
	app, tk := newApp(t, Config{Collections: []string{"_superusers", "users"}})
	recs := "/api/collections/notes/records"
	tk.MustRegisterScope(Scope{Name: "notes:read", Description: "Read notes", Paths: []string{recs}, Methods: []string{"GET"}})
	tk.MustRegisterScope(Scope{Name: "notes:write", Description: "Change notes", Paths: []string{recs}})
	// a scope that reaches the path but carries neither notes right: the rules must stop it
	tk.MustRegisterScope(Scope{Name: "notes:path_only", Paths: []string{recs}})
	tk.MustRegisterScope(Scope{Name: "echo", Paths: []string{"/api/echo"}})
	// a Go guard instead of paths
	tk.MustRegisterScope(Scope{Name: "guarded", Match: func(e *core.RequestEvent) bool {
		return e.Request.URL.Path == "/api/echo" && e.Request.URL.Query().Get("guard") == "yes"
	}})

	notes := core.NewBaseCollection("notes")
	notes.Fields.Add(&core.TextField{Name: "title"})
	notes.ListRule = types.Pointer(`@request.auth.id != "" && ` + ScopeRuleField("notes:read") + ` = "1"`)
	// no auth check on purpose: proves a client can't fake the header
	notes.CreateRule = types.Pointer(ScopeRuleField("notes:write") + ` = "1"`)
	if err := app.Save(notes); err != nil {
		t.Fatal(err)
	}
	n := core.NewRecord(notes)
	n.Set("title", "first")
	if err := app.Save(n); err != nil {
		t.Fatal(err)
	}

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/echo", func(e *core.RequestEvent) error {
			info, err := e.RequestInfo()
			if err != nil {
				return err
			}
			var held []string
			for k := range info.Headers {
				if strings.HasPrefix(k, "x_token_scope_") {
					held = append(held, strings.TrimPrefix(k, "x_token_scope_"))
				}
			}
			return e.JSON(http.StatusOK, map[string]any{"held": strings.Join(sortStrings(held), ","), "auth": e.Auth != nil})
		})
		return se.Next()
	})
	return app, tk
}

func sortStrings(s []string) []string {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return s
}

func TestScopeHeadersRulesAndMethods(t *testing.T) {
	app, tk := newNotesApp(t)
	user, _ := app.FindAuthRecordByEmail("users", "test@example.com")
	read, _, _ := tk.Create(user, CreateOptions{Name: "r", Scopes: []string{"notes:read"}})
	write, _, _ := tk.Create(user, CreateOptions{Name: "w", Scopes: []string{"notes:write"}})
	pathOnly, _, _ := tk.Create(user, CreateOptions{Name: "p", Scopes: []string{"notes:path_only"}})
	echo, _, _ := tk.Create(user, CreateOptions{Name: "e", Scopes: []string{"echo", "notes:read"}})
	guarded, _, _ := tk.Create(user, CreateOptions{Name: "g", Scopes: []string{"guarded"}})
	full, _, _ := tk.Create(user, CreateOptions{Name: "full"})
	jwt, err := user.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	recs := "/api/collections/notes/records"
	body := func() *strings.Reader { return strings.NewReader(`{"title":"x"}`) }
	allHeld := "deploy,echo,guarded,notes_path_only,notes_read,notes_write"

	factory := func(testing.TB) *tests.TestApp { return app }
	scenarios := []tests.ApiScenario{
		// rules
		{Name: "read scope lists notes", Method: http.MethodGet, URL: recs, Headers: authHeader(read),
			ExpectedStatus: 200, ExpectedContent: []string{`"totalItems":1`}},
		{Name: "read scope can't write: its methods stop it before the rules", Method: http.MethodPost, URL: recs, Body: body(),
			Headers: authHeader(read), ExpectedStatus: 403, ExpectedContent: []string{"scopes"}},
		{Name: "write scope creates", Method: http.MethodPost, URL: recs, Body: body(), Headers: authHeader(write),
			ExpectedStatus: 200, ExpectedContent: []string{`"title":"x"`}},
		{Name: "write scope doesn't hold read: the list rule filters everything", Method: http.MethodGet, URL: recs,
			Headers: authHeader(write), ExpectedStatus: 200, ExpectedContent: []string{`"totalItems":0`}},
		{Name: "a scope on the path without the right is stopped by the rule", Method: http.MethodPost, URL: recs, Body: body(),
			Headers: authHeader(pathOnly), ExpectedStatus: 400, ExpectedContent: []string{"Failed to create record"}},
		{Name: "a JWT holds every scope", Method: http.MethodPost, URL: recs, Body: body(), Headers: map[string]string{"Authorization": jwt},
			ExpectedStatus: 200, ExpectedContent: []string{`"title":"x"`}},
		{Name: "an unscoped token holds every scope", Method: http.MethodGet, URL: recs, Headers: authHeader(full),
			ExpectedStatus: 200, ExpectedContent: []string{`"title":"first"`}},
		{Name: "anonymous can't fake the header", Method: http.MethodPost, URL: recs, Body: body(),
			Headers: map[string]string{"X-Token-Scope-Notes-Write": "1", "x-token-scope-notes-write": "1"}, ExpectedStatus: 400,
			ExpectedContent: []string{"Failed to create record"}},

		// what hooks and rules see
		{Name: "a scoped token sees only its own scopes", Method: http.MethodGet, URL: "/api/echo", Headers: authHeader(echo),
			ExpectedStatus: 200, ExpectedContent: []string{`"held":"echo,notes_read"`}},
		{Name: "a JWT sees every scope", Method: http.MethodGet, URL: "/api/echo", Headers: map[string]string{"Authorization": jwt},
			ExpectedStatus: 200, ExpectedContent: []string{`"held":"` + allHeld + `"`}},
		{Name: "anonymous sees none, even when it sends some", Method: http.MethodGet, URL: "/api/echo",
			Headers: map[string]string{"X-Token-Scope-Echo": "1"}, ExpectedStatus: 200, ExpectedContent: []string{`"held":""`, `"auth":false`}},
		{Name: "a scoped token can't add scopes by sending headers", Method: http.MethodGet, URL: "/api/echo",
			Headers:        map[string]string{"Authorization": "Bearer " + echo, "X-Token-Scope-Notes-Write": "1"},
			ExpectedStatus: 200, ExpectedContent: []string{`"held":"echo,notes_read"`}},

		// Go guard
		{Name: "a Match guard lets its token in", Method: http.MethodGet, URL: "/api/echo?guard=yes", Headers: authHeader(guarded),
			ExpectedStatus: 200, ExpectedContent: []string{`"held":"guarded"`}},
		{Name: "elsewhere the guarded token is refused", Method: http.MethodGet, URL: "/api/echo", Headers: authHeader(guarded),
			ExpectedStatus: 403, ExpectedContent: []string{"scopes (guarded)"}},
	}
	for _, s := range scenarios {
		s.TestAppFactory = factory
		s.DisableTestAppCleanup = true
		s.Test(t)
	}
}

func TestScopeCollectionsLimitHeldScopes(t *testing.T) {
	app, tk := newApp(t, Config{Collections: []string{"_superusers", "users"}})
	tk.MustRegisterScope(Scope{Name: "admin_only", Collections: []string{"_superusers"}, Paths: []string{"/x"}})
	user, _ := app.FindAuthRecordByEmail("users", "test@example.com")
	e := &core.RequestEvent{}
	e.Auth = user
	if got := strings.Join(tk.heldScopes(e), ","); got != "deploy" {
		t.Fatalf("user holds %q", got)
	}
	e.Auth = superuser(t, app)
	if got := strings.Join(tk.heldScopes(e), ","); got != "admin_only,deploy" {
		t.Fatalf("superuser holds %q", got)
	}
}

func TestBindJS(t *testing.T) {
	app, tk := newApp(t, Config{})
	vm := goja.New()
	BindJS(vm, app)
	_, err := vm.RunString(`
		$tokens.registerScope({ name: "notes:read", description: "Read notes", paths: ["/api/collections/notes/records"], methods: ["get"] })
		$tokens.registerScope({ name: "notes:read", paths: ["/api/notes"] })
	`)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := tk.registry.get("notes:read")
	if !ok || s.Description != "Read notes" || strings.Join(s.Paths, ",") != "/api/collections/notes/records,/api/notes" || strings.Join(s.Methods, ",") != "GET" {
		t.Fatalf("%+v", s)
	}
	v, err := vm.RunString(`[$tokens.ruleField("notes:read"), $tokens.scopes().map((s) => s.name + "|" + s.header).join(","), typeof $tokens.requireScope("notes:read")]`)
	if err != nil {
		t.Fatal(err)
	}
	got := v.Export().([]any)
	if got[0] != "@request.headers.x_token_scope_notes_read" || got[1] != "deploy|X-Token-Scope-Deploy,notes:read|X-Token-Scope-Notes-Read" || got[2] != "object" {
		t.Fatalf("%v", got)
	}
	if _, err := vm.RunString(`$tokens.registerScope({ name: "Bad Name" })`); err == nil || !strings.Contains(err.Error(), "invalid scope name") {
		t.Fatalf("bad name: %v", err)
	}
	if _, err := vm.RunString(`$tokens.registerScope({ name: "x", paths: "nope" })`); err == nil {
		t.Fatal("paths as a string accepted")
	}
}

// An in-process request made with WithScopes is checked like a scoped token's request, whatever credential it carries.
func TestWithScopes(t *testing.T) {
	app, tk := newNotesApp(t)
	user, _ := app.FindAuthRecordByEmail("users", "test@example.com")
	jwt, err := user.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	echo, _, _ := tk.Create(user, CreateOptions{Name: "e", Scopes: []string{"echo", "notes:read"}})
	var router *router.Router[*core.RequestEvent]
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		router = se.Router
		// /api/echo/inner?scopes=a,b&method=GET&path=/x replays a request in process, limited to scopes ("-" = none)
		se.Router.Any("/api/echo/inner", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			ctx := e.Request.Context()
			for _, list := range q["scopes"] {
				var scopes []string
				if list != "-" {
					scopes = strings.Split(list, ",")
				}
				ctx = WithScopes(ctx, scopes)
			}
			mux, err := router.BuildMux()
			if err != nil {
				return err
			}
			r := httptest.NewRequestWithContext(ctx, q.Get("method"), q.Get("path"), strings.NewReader(`{"title":"x"}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", e.Request.Header.Get("Authorization"))
			r.Header.Set("X-Token-Scope-Notes-Write", "1") // can't widen
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			return e.String(http.StatusOK, fmt.Sprintf("%d %s", w.Code, w.Body.String()))
		})
		return se.Next()
	})
	recs := "/api/collections/notes/records"
	inner := func(scopes, method, path string) string {
		return "/api/echo/inner?scopes=" + scopes + "&method=" + method + "&path=" + url.QueryEscape(path)
	}
	jwtH := map[string]string{"Authorization": jwt}
	factory := func(testing.TB) *tests.TestApp { return app }
	for _, s := range []tests.ApiScenario{
		{Name: "no limit: a JWT holds every scope", Method: http.MethodGet, URL: "/api/echo/inner?method=GET&path=/api/echo", Headers: jwtH,
			ExpectedStatus: 200, ExpectedContent: []string{`200 `, `"held":"deploy,echo,guarded,notes_path_only,notes_read,notes_write"`}},
		{Name: "limited JWT sees only the limit", Method: http.MethodGet, URL: inner("echo", "GET", "/api/echo"), Headers: jwtH,
			ExpectedStatus: 200, ExpectedContent: []string{`200 `, `"held":"echo"`}},
		{Name: "limited JWT reads notes", Method: http.MethodGet, URL: inner("notes:read", "GET", recs), Headers: jwtH,
			ExpectedStatus: 200, ExpectedContent: []string{`200 `, `"totalItems":1`}},
		{Name: "read limit can't write", Method: http.MethodGet, URL: inner("notes:read", "POST", recs), Headers: jwtH,
			ExpectedStatus: 200, ExpectedContent: []string{`403 `, `connection's scopes (notes:read)`}},
		{Name: "a limit off the path is refused", Method: http.MethodGet, URL: inner("notes:read", "GET", "/api/echo"), Headers: jwtH,
			ExpectedStatus: 200, ExpectedContent: []string{`403 `}},
		{Name: "a nested limit only narrows", Method: http.MethodGet, URL: inner("echo", "GET", "/api/echo") + "&scopes=echo,notes:write", Headers: jwtH,
			ExpectedStatus: 200, ExpectedContent: []string{`200 `, `"held":"echo"`}},
		{Name: "a nested limit outside the first allows nothing", Method: http.MethodGet, URL: inner("echo", "POST", recs) + "&scopes=notes:write", Headers: jwtH,
			ExpectedStatus: 200, ExpectedContent: []string{`403 `}},
		{Name: "an empty limit allows nothing", Method: http.MethodGet, URL: inner("-", "GET", "/api/echo"), Headers: jwtH,
			ExpectedStatus: 200, ExpectedContent: []string{`403 `}},
		{Name: "a scoped token under a limit holds the overlap", Method: http.MethodGet, URL: inner("echo,notes:write", "GET", "/api/echo"),
			Headers: authHeader(echo), ExpectedStatus: 200, ExpectedContent: []string{`200 `, `"held":"echo"`}},
	} {
		s.TestAppFactory = factory
		s.DisableTestAppCleanup = true
		s.Test(t)
	}
}

func TestReachesAndInfrastructure(t *testing.T) {
	_, tk := newNotesApp(t)
	for _, c := range []struct {
		scopes          []string
		method, pattern string
		want            bool
	}{
		{[]string{"notes:read"}, "GET", "/api/collections/notes/records", true},
		{[]string{"notes:read"}, "GET", "/api/collections/notes/records/{id}", true},
		{[]string{"notes:read"}, "GET", "/api/collections/{collection}/records", true},
		{[]string{"notes:read"}, "PATCH", "/api/collections/notes/records/{id}", false},
		{[]string{"notes:read"}, "GET", "/api/collections/other/records", false},
		{[]string{"notes:read"}, "GET", "/api/collections", false},
		{[]string{"notes:read"}, "GET", "/api/health", false},
		{[]string{"notes:write"}, "PATCH", "/api/collections/{collection}/records/{id}", true},
		{[]string{"guarded"}, "DELETE", "/anything", true},
		{nil, "GET", "/api/_c/remote/v1", true},
	} {
		if got := tk.Reaches(c.scopes, c.method, c.pattern); got != c.want {
			t.Errorf("Reaches(%v, %s %s) = %v", c.scopes, c.method, c.pattern, got)
		}
	}
	if !(Scope{Paths: []string{"/api/_c/deploy/v1/"}}).Infrastructure() || (Scope{Paths: []string{"/api/x"}}).Infrastructure() ||
		(Scope{}).Infrastructure() {
		t.Fatal("Infrastructure")
	}
}

// The stock binary loads pb_hooks before tokens.Register: scopes registered then are kept for it.
func TestBindJSBeforeRegister(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	vm := goja.New()
	BindJS(vm, app)
	if _, err := vm.RunString(`$tokens.registerScope({ name: "talks:read", description: "See talks", paths: ["/api/collections/talks/records"], methods: ["GET"] })`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(`$tokens.registerScope({ name: "Bad Name" })`); err == nil {
		t.Fatal("bad name accepted before Register")
	}
	tk, err := Register(app, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := tk.registry.get("talks:read"); !ok || s.Description != "See talks" {
		t.Fatalf("pending scope lost: %+v", s)
	}
	if _, err := vm.RunString(`$tokens.registerScope({ name: "talks:write", paths: ["/api/collections/talks/records"] })`); err != nil {
		t.Fatal(err)
	}
	if _, ok := tk.registry.get("talks:write"); !ok {
		t.Fatal("scope after Register")
	}
}
