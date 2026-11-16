package mcp

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"github.com/pockethost/pocketone/ext/extkit/js"
	"github.com/pockethost/pocketone/ext/jsvm"
)

type harness struct {
	t       *testing.T
	app     *tests.TestApp
	srv     *Server
	handler http.Handler
}

func newHarness(t *testing.T, cfg Config, setup ...func(*Server)) *harness {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	s := MustRegister(app, cfg)
	for _, f := range setup {
		f(s)
	}
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, app: app, srv: s}
	ev := &core.ServeEvent{App: app, Router: r}
	if err := app.OnServe().Trigger(ev, func(e *core.ServeEvent) error {
		mux, err := e.Router.BuildMux()
		h.handler = mux
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h *harness) token(collection, email string) string {
	h.t.Helper()
	rec, err := h.app.FindAuthRecordByEmail(collection, email)
	if err != nil {
		h.t.Fatal(err)
	}
	tok, err := rec.NewAuthToken()
	if err != nil {
		h.t.Fatal(err)
	}
	return tok
}

func (h *harness) superuser() string {
	return h.token(core.CollectionNameSuperusers, "test@example.com")
}

type resp struct {
	code   int
	header http.Header
	body   map[string]any
	raw    string
}

func (h *harness) do(method, path, token, body string, headers ...string) resp {
	h.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	r := resp{code: rec.Code, header: rec.Header(), raw: rec.Body.String()}
	_ = json.Unmarshal(rec.Body.Bytes(), &r.body)
	return r
}

// rpc posts one JSON-RPC request and returns the response.
func (h *harness) rpc(token, method string, params any) resp {
	h.t.Helper()
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	return h.do(http.MethodPost, "/mcp", token, string(b))
}

func (h *harness) call(token, tool string, args map[string]any) map[string]any {
	h.t.Helper()
	r := h.rpc(token, "tools/call", map[string]any{"name": tool, "arguments": args})
	if r.code != 200 {
		h.t.Fatalf("%s: HTTP %d %s", tool, r.code, r.raw)
	}
	res, ok := r.body["result"].(map[string]any)
	if !ok {
		h.t.Fatalf("%s: no result: %s", tool, r.raw)
	}
	return res
}

func (h *harness) callOK(token, tool string, args map[string]any) map[string]any {
	h.t.Helper()
	res := h.call(token, tool, args)
	if res["isError"] == true {
		h.t.Fatalf("%s failed: %v", tool, res["content"])
	}
	sc, _ := res["structuredContent"].(map[string]any)
	return sc
}

func toolNames(r resp) []string {
	names := []string{}
	res, _ := r.body["result"].(map[string]any)
	list, _ := res["tools"].([]any)
	for _, t := range list {
		names = append(names, t.(map[string]any)["name"].(string))
	}
	return names
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func TestChallengeAndMetadata(t *testing.T) {
	h := newHarness(t, Config{PublicURL: "https://acme.example.com"})

	r := h.rpc("", "initialize", map[string]any{"protocolVersion": "2025-06-18"})
	if r.code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d %s", r.code, r.raw)
	}
	wa := r.header.Get("WWW-Authenticate")
	if !strings.Contains(wa, `resource_metadata="https://acme.example.com/.well-known/oauth-protected-resource/mcp"`) ||
		strings.Contains(wa, "scope=") || strings.Contains(wa, "invalid_token") {
		t.Fatalf("bad challenge: %s", wa)
	}

	r = h.rpc("not-a-token", "initialize", nil)
	if r.code != http.StatusUnauthorized || !strings.Contains(r.header.Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Fatalf("bad token: want 401 invalid_token, got %d %s", r.code, r.header.Get("WWW-Authenticate"))
	}

	for _, path := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource"} {
		r = h.do(http.MethodGet, path, "", "")
		if r.code != 200 || r.body["resource"] != "https://acme.example.com/mcp" {
			t.Fatalf("%s: %d %s", path, r.code, r.raw)
		}
		as, _ := r.body["authorization_servers"].([]any)
		if len(as) != 1 || as[0] != "https://acme.example.com" {
			t.Fatalf("%s: authorization_servers %v", path, as)
		}
	}

	// without PublicURL the resource follows the host the client used, not the default app URL
	h2 := newHarness(t, Config{})
	req := h2.do(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", "", "", "X-Forwarded-Proto", "https")
	if req.body["resource"] != "https://example.com/mcp" {
		t.Fatalf("derived resource: %s", req.raw)
	}

	if r = h.do(http.MethodGet, "/mcp", h.superuser(), ""); r.code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: want 405, got %d", r.code)
	}
}

func TestProtocol(t *testing.T) {
	h := newHarness(t, Config{Name: "acme", Version: "1.2.3", Instructions: "Be nice."}, func(s *Server) {
		s.AddInstructions("Extra notes.")
	})
	tok := h.superuser()

	r := h.rpc(tok, "initialize", map[string]any{"protocolVersion": "2025-06-18"})
	res := r.body["result"].(map[string]any)
	if res["protocolVersion"] != "2025-06-18" {
		t.Fatalf("version not echoed: %v", res["protocolVersion"])
	}
	info := res["serverInfo"].(map[string]any)
	if info["name"] != "acme" || info["version"] != "1.2.3" || info["title"] == "" {
		t.Fatalf("serverInfo: %v", info)
	}
	if res["instructions"] != "Be nice.\n\nExtra notes." { // no own tools: no "prefer app tools" paragraph
		t.Fatalf("instructions: %q", res["instructions"])
	}

	r = h.rpc(tok, "initialize", map[string]any{"protocolVersion": "1999-01-01"})
	if v := r.body["result"].(map[string]any)["protocolVersion"]; v != SupportedVersions[0] {
		t.Fatalf("unknown version: want newest, got %v", v)
	}

	if r = h.rpc(tok, "ping", nil); r.code != 200 || r.body["result"] == nil {
		t.Fatalf("ping: %s", r.raw)
	}

	if r = h.do(http.MethodPost, "/mcp", tok, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); r.code != http.StatusAccepted {
		t.Fatalf("notification: want 202, got %d", r.code)
	}
	if r = h.do(http.MethodPost, "/mcp", tok, `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`); r.code != http.StatusBadRequest {
		t.Fatalf("batch: want 400, got %d", r.code)
	}
	if r = h.do(http.MethodPost, "/mcp", tok, `{nope`); r.code != http.StatusBadRequest {
		t.Fatalf("parse error: want 400, got %d", r.code)
	}
	if r = h.do(http.MethodPost, "/mcp", tok, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, "MCP-Protocol-Version", "1999-01-01"); r.code != http.StatusBadRequest {
		t.Fatalf("bad protocol header: want 400, got %d", r.code)
	}
	if r = h.do(http.MethodPost, "/mcp", tok, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, "MCP-Protocol-Version", "2025-11-25"); r.code != 200 {
		t.Fatalf("good protocol header: want 200, got %d", r.code)
	}
	if r = h.do(http.MethodPost, "/mcp", tok, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, "Origin", "https://evil.example"); r.code != http.StatusForbidden {
		t.Fatalf("foreign origin: want 403, got %d", r.code)
	}
	r = h.do(http.MethodPost, "/mcp", tok, `{"jsonrpc":"2.0","id":"abc","method":"resources/list"}`)
	if e, _ := r.body["error"].(map[string]any); e == nil || e["code"] != float64(codeMethodNotFound) || r.body["id"] != "abc" {
		t.Fatalf("unknown method: %s", r.raw)
	}

	r = h.rpc(tok, "tools/call", map[string]any{"name": "nope"})
	if e, _ := r.body["error"].(map[string]any); e == nil || e["code"] != float64(codeInvalidParams) {
		t.Fatalf("unknown tool: %s", r.raw)
	}
}

// notesApp adds a "notes" collection with owner rules, a protected file field and a request hook, plus a custom
// route behind a middleware; tools must see all of it.
func notesApp(s *Server) {
	app := s.App()
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		panic(err)
	}
	c := core.NewBaseCollection("notes")
	own := "owner = @request.auth.id"
	create := `@request.auth.id != "" && owner = @request.auth.id`
	c.ListRule, c.ViewRule, c.UpdateRule, c.DeleteRule, c.CreateRule = &own, &own, &own, &own, &create
	c.Fields.Add(
		&core.RelationField{Name: "owner", CollectionId: users.Id, MaxSelect: 1},
		&core.TextField{Name: "title", Required: true},
		&core.TextField{Name: "stamp"},
		&core.FileField{Name: "attachment", MaxSelect: 1, MaxSize: 1 << 20, Protected: true},
	)
	if err := app.Save(c); err != nil {
		panic(err)
	}
	// a request hook: only runs for REST requests, so it proves tool calls are REST requests
	app.OnRecordCreateRequest("notes").BindFunc(func(e *core.RecordRequestEvent) error {
		e.Record.Set("stamp", "via-request-hook")
		return e.Next()
	})
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/notes/archive/{id}", func(e *core.RequestEvent) error {
			return e.JSON(200, map[string]any{"archived": e.Request.PathValue("id"), "by": e.Auth.Email()})
		}).BindFunc(func(e *core.RequestEvent) error { // a middleware that blocks one value
			if e.Request.PathValue("id") == "blocked" {
				return e.ForbiddenError("blocked by middleware", nil)
			}
			return e.Next()
		}).Bind(apis.RequireAuth())
		return se.Next()
	})
	s.MustAddEndpoint(Endpoint{Name: "notes_archive_note", Description: "Archive a note.", Method: "POST",
		Path: "/api/notes/archive/{id}", InputSchema: map[string]any{"type": "object", "required": []any{"id"},
			"properties": map[string]any{"id": map[string]any{"type": "string"}}}})
}

func TestToolsAreRESTRequests(t *testing.T) {
	h := newHarness(t, Config{}, notesApp)
	user := h.token("users", "test@example.com")
	other := h.token("users", "test2@example.com")
	userRec, _ := h.app.FindAuthRecordByEmail("users", "test@example.com")

	// a user creates their own note; the create rule allows it and the request hook runs
	n := h.callOK(user, "pb_create_record", map[string]any{"collection": "notes", "data": map[string]any{
		"owner": userRec.Id, "title": "mine"}})
	id, _ := n["id"].(string)
	if id == "" || n["stamp"] != "via-request-hook" {
		t.Fatalf("create as user: %v", n)
	}
	// the create rule refuses a note for someone else
	if res := h.call(other, "pb_create_record", map[string]any{"collection": "notes", "data": map[string]any{
		"owner": userRec.Id, "title": "not mine"}}); res["isError"] != true || !strings.Contains(textOf(res), "HTTP 400") {
		t.Fatalf("create for another user should fail: %v", res)
	}
	// list rule: the other user sees nothing, the owner sees one, the superuser sees all
	if l := h.callOK(other, "pb_list_records", map[string]any{"collection": "notes"}); l["totalItems"] != float64(0) {
		t.Fatalf("other's list: %v", l)
	}
	l := h.callOK(user, "pb_list_records", map[string]any{"collection": "notes"})
	if l["totalItems"] != float64(1) || l["perPage"] != float64(20) {
		t.Fatalf("owner's list (perPage defaults to 20): %v", l)
	}
	if l := h.callOK(h.superuser(), "pb_list_records", map[string]any{"collection": "notes", "perPage": 5}); l["perPage"] != float64(5) {
		t.Fatalf("superuser list: %v", l)
	}
	if res := h.call(other, "pb_update_record", map[string]any{"collection": "notes", "id": id, "data": map[string]any{"title": "x"}}); res["isError"] != true {
		t.Fatalf("other's update should 404: %v", res)
	}

	// a user can't reach superuser endpoints, and doesn't see their tools listed
	if res := h.call(user, "pb_list_collections", nil); res["isError"] != true || !strings.Contains(textOf(res), "HTTP 403") {
		t.Fatalf("user listing collections: %v", res)
	}
	names := toolNames(h.rpc(user, "tools/list", nil))
	if contains(names, "pb_list_collections") || !contains(names, "pb_list_records") || names[0] != "notes_archive_note" {
		t.Fatalf("user's tools (own first, no superuser tools): %v", names)
	}
	if !contains(toolNames(h.rpc(h.superuser(), "tools/list", nil)), "pb_run_sql") {
		t.Fatal("superuser should see pb_run_sql")
	}

	// a custom route: path param from args, route middleware applies, runs as the caller
	if a := h.callOK(user, "notes_archive_note", map[string]any{"id": id}); a["archived"] != id || a["by"] != "test@example.com" {
		t.Fatalf("custom tool: %v", a)
	}
	if res := h.call(user, "notes_archive_note", map[string]any{"id": "blocked"}); !strings.Contains(textOf(res), "Blocked by middleware") {
		t.Fatalf("middleware should block: %v", res)
	}
	for _, bad := range []string{"..", ".", ""} {
		if res := h.call(user, "notes_archive_note", map[string]any{"id": bad}); res["isError"] != true || !strings.Contains(textOf(res), "invalid") {
			t.Fatalf("path value %q should be refused: %v", bad, res)
		}
	}
	if res := h.call(user, "notes_archive_note", map[string]any{}); !strings.Contains(textOf(res), "missing required") {
		t.Fatalf("schema check: %v", res)
	}

	// instructions point agents at the app's own tools
	init := h.rpc(user, "initialize", nil).body["result"].(map[string]any)
	if !strings.Contains(init["instructions"].(string), "Use them when one fits") {
		t.Fatalf("instructions: %v", init["instructions"])
	}
}

func TestFiles(t *testing.T) {
	h := newHarness(t, Config{}, notesApp)
	user := h.token("users", "test@example.com")
	other := h.token("users", "test2@example.com")
	userRec, _ := h.app.FindAuthRecordByEmail("users", "test@example.com")

	text := "hello from a file"
	n := h.callOK(user, "pb_create_record", map[string]any{"collection": "notes", "data": map[string]any{
		"owner": userRec.Id, "title": "with file",
		"attachment": map[string]any{"name": "hello.txt", "base64": base64.StdEncoding.EncodeToString([]byte(text)), "type": "text/plain"},
	}})
	fname, _ := n["attachment"].(string)
	if fname == "" || n["stamp"] != "via-request-hook" {
		t.Fatalf("upload: %v", n)
	}

	// download as the owner (protected: a file token is made as the caller)
	res := h.call(user, "pb_download_file", map[string]any{"collection": "notes", "recordId": n["id"], "filename": fname})
	content := res["content"].([]any)[0].(map[string]any)
	r, _ := content["resource"].(map[string]any)
	if res["isError"] == true || content["type"] != "resource" || r["text"] != text {
		t.Fatalf("download: %v", res)
	}
	// another user's file token doesn't open it
	if res := h.call(other, "pb_download_file", map[string]any{"collection": "notes", "recordId": n["id"], "filename": fname}); res["isError"] != true {
		t.Fatalf("other user downloading: %v", res)
	}

	// images come back as images
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")
	n2 := h.callOK(user, "pb_create_record", map[string]any{"collection": "notes", "data": map[string]any{
		"owner": userRec.Id, "title": "pic", "attachment": map[string]any{"name": "dot.png", "base64": base64.StdEncoding.EncodeToString(png)}}})
	res = h.call(user, "pb_download_file", map[string]any{"collection": "notes", "recordId": n2["id"], "filename": n2["attachment"]})
	if c := res["content"].([]any)[0].(map[string]any); c["type"] != "image" || c["mimeType"] != "image/png" {
		t.Fatalf("image download: %v", res)
	}

	// over the file cap
	h2 := newHarness(t, Config{MaxFile: 8}, notesApp)
	if res := h2.call(h2.token("users", "test@example.com"), "pb_create_record", map[string]any{"collection": "notes",
		"data": map[string]any{"title": "big", "attachment": map[string]any{"name": "big.txt", "base64": base64.StdEncoding.EncodeToString([]byte(text))}}}); !strings.Contains(textOf(res), "limit") {
		t.Fatalf("file over the cap: %v", res)
	}
}

func TestSuperuserBuiltins(t *testing.T) {
	h := newHarness(t, Config{})
	tok := h.superuser()

	h.callOK(tok, "pb_create_collection", map[string]any{"data": map[string]any{
		"name": "mcp_posts", "type": "base",
		"fields": []any{map[string]any{"name": "title", "type": "text", "required": true}},
	}})
	if c := h.callOK(tok, "pb_get_collection", map[string]any{"collection": "mcp_posts"}); c["name"] != "mcp_posts" {
		t.Fatalf("get collection: %v", c)
	}
	if res := h.call(tok, "pb_create_record", map[string]any{"collection": "mcp_posts", "data": map[string]any{}}); res["isError"] != true {
		t.Fatalf("validation error should be isError: %v", res)
	}
	if res := h.call(tok, "pb_delete_collection", map[string]any{"collection": "mcp_posts", "confirm": "wrong"}); res["isError"] != true {
		t.Fatalf("bad confirm should fail: %v", res)
	}
	h.call(tok, "pb_delete_collection", map[string]any{"collection": "mcp_posts", "confirm": "mcp_posts"})
	if _, err := h.app.FindCollectionByNameOrId("mcp_posts"); err == nil {
		t.Fatal("collection should be gone")
	}

	h.callOK(tok, "pb_update_settings", map[string]any{"data": map[string]any{"meta": map[string]any{"appName": "Acme MCP"}}})
	if h.app.Settings().Meta.AppName != "Acme MCP" {
		t.Fatalf("settings not updated")
	}
	h.callOK(tok, "pb_get_settings", nil)
	h.callOK(tok, "pb_list_logs", nil)
	h.callOK(tok, "pb_check_health", nil)
	sql := h.callOK(tok, "pb_run_sql", map[string]any{"query": "SELECT 1 AS one"})
	if rows, _ := sql["rows"].([]any); len(rows) != 1 {
		t.Fatalf("sql: %v", sql)
	}
	if res := h.call(tok, "pb_list_collections", map[string]any{"bogus": 1}); !strings.Contains(textOf(res), "unknown argument") {
		t.Fatalf("unknown args: %v", res)
	}
}

func TestResponseCap(t *testing.T) {
	h := newHarness(t, Config{MaxResponse: 200})
	res := h.call(h.superuser(), "pb_list_collections", nil)
	if res["isError"] == true || res["structuredContent"] != nil || !strings.Contains(textOf(res), "[cut at") {
		t.Fatalf("cut response: %v", res)
	}
}

func TestReadOnlyBuiltinsAndGroups(t *testing.T) {
	h := newHarness(t, Config{BuiltinGroups: []string{"records"}, ReadOnlyBuiltins: true})
	names := toolNames(h.rpc(h.superuser(), "tools/list", nil))
	if len(names) != 2 || !contains(names, "pb_list_records") || !contains(names, "pb_get_record") {
		t.Fatalf("read-only records tools: %v", names)
	}
}

func TestBindJS(t *testing.T) {
	h := newHarness(t, Config{}, notesApp)
	vm := jsVM(t, func(rt js.Runtime) { BindJS(h.app, rt) })
	if _, err := vm.RunString(`
		$mcp.addEndpoint({
			name: "notes_archive_js",
			description: "Archive via JS.",
			method: "POST",
			path: "/api/notes/archive/{id}",
			request: ({ args, request }) => ({ ...request, path: "/api/notes/archive/" + $mcp.pathValue(args.id + "-js") }),
			response: ({ status, json }) => { if (status !== 200) throw new Error("nope " + status); return { ok: json.archived } },
		})
		$mcp.instructions("JS notes.")
	`); err != nil {
		t.Fatal(err)
	}
	user := h.token("users", "test@example.com")
	if r := h.callOK(user, "notes_archive_js", map[string]any{"id": "a"}); r["ok"] != "a-js" {
		t.Fatalf("js tool: %v", r)
	}
	if res := h.call(user, "notes_archive_js", map[string]any{"id": "blocked"}); res["isError"] == true {
		t.Fatalf("blocked-js isn't blocked: %v", res)
	}
	if _, err := vm.RunString(`$mcp.addEndpoint({ name: "x_y_z" })`); err == nil {
		t.Fatal("addEndpoint without method/path should throw")
	}
	if v, _ := vm.RunString(`$mcp.tools()[0]`); v.String() != "notes_archive_js" && v.String() != "notes_archive_note" {
		t.Fatalf("tools(): own tools first, got %v", v)
	}
}

func textOf(res map[string]any) string {
	var b strings.Builder
	list, _ := res["content"].([]any)
	for _, c := range list {
		if m, ok := c.(map[string]any); ok {
			if s, ok := m["text"].(string); ok {
				b.WriteString(s)
			}
		}
	}
	return b.String()
}

// jsvm runs pb_hooks when it's registered, which can be before mcp.Register (as in the stock binary).
func TestBindJSBeforeRegister(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	vm := jsVM(t, func(rt js.Runtime) { BindJS(app, rt) })
	if _, err := vm.RunString(`$mcp.addEndpoint({ name: "early_get_thing", description: "x", method: "GET", path: "/api/health" }); $mcp.instructions("Early.")`); err != nil {
		t.Fatal(err)
	}
	s := MustRegister(app, Config{})
	if s.tool("early_get_thing") == nil || !strings.Contains(s.instructionsText(), "Early.") {
		t.Fatal("tool declared before Register is missing")
	}
}

// jsVM is an ext/jsvm VM set up by bind.
func jsVM(t testing.TB, bind func(js.Runtime)) *jsvm.VM {
	t.Helper()
	vm, err := jsvm.New(jsvm.VMConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vm.Close)
	if err := vm.Run(func(rt js.Runtime) error { bind(rt); return nil }); err != nil {
		t.Fatal(err)
	}
	return vm
}
