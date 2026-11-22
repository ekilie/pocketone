package kv_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	kvv1 "pocketone.dev/ext/contracts/kv/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/extkit/js"
	"pocketone.dev/ext/jsvm"
	"pocketone.dev/ext/kv"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension { return kv.New() })
}

func newApp(t *testing.T) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	if _, err := extkit.Setup(app, []extkit.Extension{kv.New()}, nil, nil); err != nil {
		t.Fatal(err)
	}
	return app
}

type settings struct {
	Title   string `json:"title"`
	Minutes int    `json:"minutes"`
}

// The helpers: a value round-trips, a missing name is not found, names stay apart from PocketBase's own params.
func TestHelpers(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	var s settings
	if found, err := kvv1.Get(app, "storytime/settings", &s); err != nil || found {
		t.Fatalf("missing: found=%v err=%v", found, err)
	}
	if err := kvv1.Set(app, "storytime/settings", settings{"Story Time", 10}); err != nil {
		t.Fatal(err)
	}
	if found, err := kvv1.Get(app, "storytime/settings", &s); err != nil || !found || s != (settings{"Story Time", 10}) {
		t.Fatalf("round trip: %+v found=%v err=%v", s, found, err)
	}
	first, _, _ := kvv1.Lookup(app, "storytime/settings")
	if err := kvv1.Set(app, "storytime/settings", settings{"Bedtime", 5}); err != nil {
		t.Fatal(err)
	}
	e, found, err := kvv1.Lookup(app, "storytime/settings")
	if err != nil || !found || string(e.Value) != `{"title":"Bedtime","minutes":5}` || !e.Created.Equal(first.Created) || e.Updated.Before(first.Updated) {
		t.Fatalf("replace: %+v %v %v (first %+v)", e, found, err, first)
	}

	// stored as _params row kv:<name>; PocketBase's own "settings" row is untouched and unreachable
	p := &core.Param{}
	if err := app.ModelQuery(p).Model("kv:storytime/settings", p); err != nil {
		t.Fatalf("the _params row: %v", err)
	}
	if err := kvv1.Set(app, "settings", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	if err := app.ReloadSettings(); err != nil {
		t.Fatalf("PocketBase's settings after kv set \"settings\": %v", err)
	}
	if app.Settings().Meta.AppName == "" {
		t.Fatal("PocketBase's settings were overwritten")
	}

	_ = kvv1.Set(app, "storytime/voice", "alloy")
	_ = kvv1.Set(app, "other", 1)
	l, err := kvv1.List(app, "storytime/")
	if err != nil || len(l) != 2 || l[0].Name != "storytime/settings" || l[1].Name != "storytime/voice" {
		t.Fatalf("list storytime/: %+v %v", l, err)
	}
	if all, _ := kvv1.List(app, ""); len(all) != 4 {
		t.Fatalf("list all: %+v", all)
	}

	if err := kvv1.Delete(app, "storytime/voice"); err != nil {
		t.Fatal(err)
	}
	if err := kvv1.Delete(app, "storytime/voice"); err != nil {
		t.Fatalf("deleting twice: %v", err)
	}
	if _, found, _ := kvv1.Lookup(app, "storytime/voice"); found {
		t.Fatal("deleted value still there")
	}

	for _, bad := range []string{"", "/lead", "has space", "a%b", "ünïcode", strings.Repeat("a", 129)} {
		if err := kvv1.Set(app, bad, 1); !errors.Is(err, kvv1.ErrInvalidName) {
			t.Errorf("name %q: %v", bad, err)
		}
	}
	if _, err := kvv1.SetRaw(app, "x", json.RawMessage(`{nope`)); !errors.Is(err, kvv1.ErrInvalidValue) {
		t.Errorf("bad JSON: %v", err)
	}
	if _, err := kvv1.List(app, "a%"); !errors.Is(err, kvv1.ErrInvalidName) {
		t.Errorf("bad prefix: %v", err)
	}
}

// Through the contract: superusers only, a missing name is not_found.
func TestContract(t *testing.T) {
	app := newApp(t)
	s := kvv1.Contract.Must(app)
	ctx, su := context.Background(), extkit.Auth{Kind: "superuser"}
	if _, err := s.Set(ctx, su, kvv1.SetInput{Name: "a/b", Value: json.RawMessage(`[1, 2]`)}); err != nil {
		t.Fatal(err)
	}
	if e, err := s.Get(ctx, su, kvv1.Key{Name: "a/b"}); err != nil || string(e.Value) != `[1,2]` {
		t.Fatalf("get: %s %v", e.Value, err)
	}
	if l, err := s.List(ctx, su, kvv1.ListInput{Prefix: "a/"}); err != nil || len(l.Entries) != 1 {
		t.Fatalf("list: %+v %v", l, err)
	}
	if err := s.Delete(ctx, su, kvv1.Key{Name: "a/b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, su, kvv1.Key{Name: "a/b"}); !errors.Is(err, kvv1.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := s.Get(ctx, extkit.Auth{Kind: "record", Collection: "users"}, kvv1.Key{Name: "a/b"}); !errors.Is(err, extkit.ErrNotSuperuser) {
		t.Fatalf("a user: %v", err)
	}
}

// Over HTTP at /api/_c/kv/v1: a superuser sets, gets and lists; a guest is refused.
func TestHTTP(t *testing.T) {
	factory := func(t testing.TB) *tests.TestApp {
		app, err := tests.NewTestApp()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := extkit.Setup(app, []extkit.Extension{kv.New()}, nil, nil); err != nil {
			t.Fatal(err)
		}
		_ = kvv1.Set(app, "seed/one", map[string]int{"n": 1})
		return app
	}
	token := func(t testing.TB) string {
		app, _ := tests.NewTestApp()
		defer app.Cleanup()
		su, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test@example.com")
		if err != nil {
			t.Fatal(err)
		}
		tok, err := su.NewAuthToken()
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}(t)
	auth := map[string]string{"Authorization": token}
	for _, sc := range []tests.ApiScenario{
		{Name: "get", Method: http.MethodGet, URL: "/api/_c/kv/v1/get?name=seed/one", Headers: auth, ExpectedStatus: 200, ExpectedContent: []string{`"name":"seed/one"`, `"value":{"n":1}`}, TestAppFactory: factory},
		{Name: "get missing", Method: http.MethodGet, URL: "/api/_c/kv/v1/get?name=seed/none", Headers: auth, ExpectedStatus: 404, ExpectedContent: []string{"not_found"}, TestAppFactory: factory},
		{Name: "set", Method: http.MethodPost, URL: "/api/_c/kv/v1/set", Body: strings.NewReader(`{"name":"seed/two","value":"hi"}`), Headers: auth, ExpectedStatus: 200, ExpectedContent: []string{`"value":"hi"`}, TestAppFactory: factory},
		{Name: "list", Method: http.MethodGet, URL: "/api/_c/kv/v1?prefix=seed/", Headers: auth, ExpectedStatus: 200, ExpectedContent: []string{`"entries":[{"name":"seed/one"`}, TestAppFactory: factory},
		{Name: "bad name", Method: http.MethodPost, URL: "/api/_c/kv/v1/set", Body: strings.NewReader(`{"name":"no spaces","value":1}`), Headers: auth, ExpectedStatus: 400, ExpectedContent: []string{"invalid_name"}, TestAppFactory: factory},
		{Name: "guest", Method: http.MethodGet, URL: "/api/_c/kv/v1/get?name=seed/one", ExpectedStatus: 401, ExpectedContent: []string{"unauthorized"}, TestAppFactory: factory},
	} {
		sc.Test(t)
	}
}

const hooks = `onRecordCreate((e) => {
  const missing = $kv.get("js/none")
  $kv.set("js/settings", { title: "Story Time", minutes: 10, voices: ["alloy"] })
  const back = $kv.get("js/settings")
  const fromGo = $kv.get("go/value")
  $kv.set("js/n", 42)
  const names = $kv.list("js/").map((x) => x.name)
  $kv.delete("js/n")
  let badName = ""
  try { $kv.set("bad name", 1) } catch (err) { badName = String(err) }
  let privateErr = ""
  try { $kv.get("_go/secret") } catch (err) { privateErr = String(err) }
  e.record.set("result", JSON.stringify({ missing, back, fromGo, names, after: $kv.get("js/n"), badName, privateErr,
    listed: $kv.list().map((x) => x.name) }))
  e.next()
}, "probes")
`

// $kv in pb_hooks: values round-trip through JSON, a missing name is null, Go and JS see the same store.
func TestJS(t *testing.T) {
	app := newApp(t)
	c := core.NewBaseCollection("probes")
	c.Fields.Add(&core.TextField{Name: "result", Max: 10000})
	if err := app.Save(c); err != nil {
		t.Fatal(err)
	}
	if err := kvv1.Set(app, "go/value", map[string]bool{"ok": true}); err != nil {
		t.Fatal(err)
	}
	_ = kvv1.Set(app, "_go/secret", "s")
	dir := filepath.Join(t.TempDir(), "pb_hooks")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "main.pb.js"), []byte(hooks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := jsvm.Register(app, jsvm.Config{HooksDir: dir, OnInit: func(rt js.Runtime) { kv.New().BindJS(app, rt) }}); err != nil {
		t.Fatal(err)
	}
	if err := app.OnBootstrap().Trigger(&core.BootstrapEvent{App: app}, func(*core.BootstrapEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(c)
	if err := app.Save(r); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Missing    any
		Back       settings
		FromGo     map[string]bool
		Names      []string
		After      any
		BadName    string
		PrivateErr string
		Listed     []string
	}
	if err := json.Unmarshal([]byte(r.GetString("result")), &out); err != nil {
		t.Fatalf("result %q: %v", r.GetString("result"), err)
	}
	if out.Missing != nil || out.Back != (settings{"Story Time", 10}) || !out.FromGo["ok"] ||
		strings.Join(out.Names, ",") != "js/n,js/settings" || out.After != nil || !strings.Contains(out.BadName, "invalid_name") && !strings.Contains(out.BadName, "a name is") {
		t.Fatalf("from JS: %+v", out)
	}
	if !strings.Contains(out.PrivateErr, "private") || strings.Contains(strings.Join(out.Listed, ","), "_go") {
		t.Fatalf("private names from JS: %+v", out)
	}
	var s struct {
		Voices []string `json:"voices"`
	}
	if found, err := kvv1.Get(app, "js/settings", &s); err != nil || !found || len(s.Voices) != 1 {
		t.Fatalf("JS's value from Go: %+v %v %v", s, found, err)
	}
}

// Private names (starting with _): the helpers reach them; the wire API and $kv don't, and List leaves them out.
func TestPrivateNames(t *testing.T) {
	app := newApp(t)
	if err := kvv1.Set(app, "_ext/secret", map[string]string{"key": "k"}); err != nil {
		t.Fatal(err)
	}
	_ = kvv1.Set(app, "ext/open", 1)
	var v map[string]string
	if found, err := kvv1.Get(app, "_ext/secret", &v); err != nil || !found || v["key"] != "k" {
		t.Fatalf("helpers: %v %v %v", v, found, err)
	}
	for _, bad := range []string{"_", "__x", "_/x"} {
		if err := kvv1.ValidName(bad); !errors.Is(err, kvv1.ErrInvalidName) {
			t.Errorf("name %q: %v", bad, err)
		}
	}
	s := kvv1.Contract.Must(app)
	ctx, su := context.Background(), extkit.Auth{Kind: "superuser"}
	if _, err := s.Get(ctx, su, kvv1.Key{Name: "_ext/secret"}); !errors.Is(err, kvv1.ErrPrivate) {
		t.Errorf("get: %v", err)
	}
	if _, err := s.Set(ctx, su, kvv1.SetInput{Name: "_ext/secret", Value: json.RawMessage(`1`)}); !errors.Is(err, kvv1.ErrPrivate) {
		t.Errorf("set: %v", err)
	}
	if err := s.Delete(ctx, su, kvv1.Key{Name: "_ext/secret"}); !errors.Is(err, kvv1.ErrPrivate) {
		t.Errorf("delete: %v", err)
	}
	if l, err := s.List(ctx, su, kvv1.ListInput{}); err != nil || len(l.Entries) != 1 || l.Entries[0].Name != "ext/open" {
		t.Errorf("list: %+v %v", l, err)
	}
	if _, err := s.List(ctx, su, kvv1.ListInput{Prefix: "_"}); !errors.Is(err, kvv1.ErrPrivate) {
		t.Errorf("list _: %v", err)
	}
}

// MoveParam moves an extension's own pre-kv _params row once, keeping its value and times.
func TestMoveParam(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	old := &core.Param{Value: []byte(`{"secret":"s"}`)}
	old.Id = "myext"
	old.MarkAsNew()
	if err := app.Save(old); err != nil {
		t.Fatal(err)
	}
	if moved, err := kvv1.MoveParam(app, "myext", "_myext/settings"); err != nil || !moved {
		t.Fatalf("move: %v %v", moved, err)
	}
	var v struct{ Secret string }
	if found, _ := kvv1.Get(app, "_myext/settings", &v); !found || v.Secret != "s" {
		t.Fatalf("moved value: %+v", v)
	}
	if err := app.ModelQuery(&core.Param{}).Model("myext", &core.Param{}); err == nil {
		t.Fatal("the old row is still there")
	}
	if moved, err := kvv1.MoveParam(app, "myext", "_myext/settings"); err != nil || moved {
		t.Fatalf("second move: %v %v", moved, err)
	}
	if err := kvv1.MoveParamOnce(app, "myext", "_myext/settings"); err != nil {
		t.Fatal(err)
	}
	// a leftover row never overwrites the moved value
	again := &core.Param{Value: []byte(`{"secret":"stale"}`)}
	again.Id = "myext"
	again.MarkAsNew()
	_ = app.Save(again)
	if _, err := kvv1.MoveParam(app, "myext", "_myext/settings"); err != nil {
		t.Fatal(err)
	}
	if _, _ = kvv1.Get(app, "_myext/settings", &v); v.Secret != "s" {
		t.Fatalf("a leftover row replaced the value: %+v", v)
	}
	for _, from := range []string{"settings", "kv:x", ""} {
		if _, err := kvv1.MoveParam(app, from, "x"); err == nil {
			t.Errorf("moved %q", from)
		}
	}
}
