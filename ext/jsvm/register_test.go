package jsvm

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"pocketone.dev/ext/extkit/js"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func serve(t *testing.T, app core.App) http.Handler {
	t.Helper()
	return setupRouter(t, app)
}

// Register loads hooks and migrations from disk: shared top-level scope, require (relative, node_modules, json),
// process.env, Buffer, a JS migration on the app's own list, and js.Run.
func TestRegisterFromDisk(t *testing.T) {
	// a plain app's migrations list is the process-wide one: keep this test's migration out of later apps
	saved := core.AppMigrations
	t.Cleanup(func() { core.AppMigrations = saved })
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	dir := t.TempDir()
	t.Setenv("JSVM_TEST_ENV", "from-env")
	writeFiles(t, dir, map[string]string{
		"pb_hooks/a_helpers.pb.js": `
			function shout(s) { return s.toUpperCase() + "!" }
			let hits = 0
		`,
		"pb_hooks/b_routes.pb.js": `
			const greet = require("./lib/greet.js")
			const pad = require("left-pad")
			const conf = require("./lib/conf.json")
			routerAdd("GET", "/t/{name}", (e) => {
				hits++
				return e.json(200, {
					msg: shout(greet(e.request.pathValue("name"))),
					pad: pad("7", 3, "0"),
					conf: conf.name,
					env: process.env.JSVM_TEST_ENV,
					b64: Buffer.from("hi there").toString("base64"),
					back: Buffer.from("aGkgdGhlcmU=", "base64").toString(),
					hits,
				})
			})
		`,
		"pb_hooks/lib/greet.js":                       `module.exports = (n) => "hello " + n`,
		"pb_hooks/lib/conf.json":                      `{"name": "jsonconf"}`,
		"pb_hooks/node_modules/left-pad/package.json": `{"main": "lib/index.js"}`,
		"pb_hooks/node_modules/left-pad/lib/index.js": `module.exports = (s, n, c) => { s = String(s); while (s.length < n) s = c + s; return s }`,
		"pb_migrations/1700000000_jsvm_test.js": `
			const helper = "only in this file's scope"
			migrate((app) => {
				const col = new Collection({ name: "jsvm_migrated", type: "base", fields: [{ name: "title", type: "text" }] })
				app.save(col)
			}, (app) => {
				app.delete(app.findCollectionByNameOrId("jsvm_migrated"))
			})
		`,
		"pb_migrations/1700000001_jsvm_test2.js": `const helper = "no clash with the other file"; migrate((app) => {})`,
	})
	if err := Register(app, Config{HooksDir: filepath.Join(dir, "pb_hooks"), MigrationsDir: filepath.Join(dir, "pb_migrations"), TypesDir: dir}); err != nil {
		t.Fatal(err)
	}

	if err := app.RunAppMigrations(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindCollectionByNameOrId("jsvm_migrated"); err != nil {
		t.Fatalf("migration didn't run: %v", err)
	}

	h := serve(t, app)
	code, body := get(t, h, "GET", "/t/ben")
	for _, want := range []string{`"msg":"HELLO BEN!"`, `"pad":"007"`, `"conf":"jsonconf"`, `"env":"from-env"`, `"b64":"aGkgdGhlcmU="`, `"back":"hi there"`, `"hits":1`} {
		if code != 200 || !strings.Contains(body, want) {
			t.Fatalf("want %s: %d %s", want, code, body)
		}
	}
	if _, body = get(t, h, "GET", "/t/x"); !strings.Contains(body, `"hits":2`) {
		t.Fatalf("state isn't shared: %s", body)
	}

	var got any
	if err := js.Run(app, func(rt js.Runtime) error {
		got = rt.Get("hits").Export()
		return nil
	}); err != nil || got != int64(2) {
		t.Fatalf("js.Run: %v %v", got, err)
	}
}

type testExt struct {
	inits, calls, loads atomic.Int32
	client              *http.Client
}

func (x *testExt) OnInit(rt js.Runtime) {
	x.inits.Add(1)
	_ = rt.Set("fromExt", js.NativeFunc(func(c js.Call) (any, error) {
		return "ext:" + c.Arg(0).String(), nil
	}))
}

func (x *testExt) OnCall(rt js.Runtime) (func(), error) {
	x.calls.Add(1)
	return func() {}, nil
}

func (x *testExt) HTTPClient(rt js.Runtime) *http.Client { return x.client }

func (x *testExt) HookFiles(pattern string) (map[string][]byte, error) {
	return map[string][]byte{"main.pb.js": []byte(`
		const lib = require("./lib.js")
		routerAdd("GET", "/ext", (e) => e.string(200, fromExt(lib.name) + " " + $bound.ping()))
		onBootstrap((e) => { e.next() })
	`)}, nil
}

func (x *testExt) Modules() (string, func(string) ([]byte, error)) {
	fsys := fstest.MapFS{"lib.js": {Data: []byte(`module.exports = { name: "embedded" }`)}}
	return "/embedded", func(p string) ([]byte, error) {
		return fsys.ReadFile(strings.TrimPrefix(p, "/embedded/"))
	}
}

func (x *testExt) OnLoad(rt js.Runtime, root, file string, content []byte) (func(), func(error) error) {
	x.loads.Add(1)
	return nil, nil
}

func (x *testExt) BindJS(app core.App, rt js.Runtime) {
	_ = rt.Set("$bound", map[string]any{"ping": func() string { return "pong" }})
}

// The extkit/js extension methods: OnInit, OnCall, HTTPClient, HookFiles, Modules, OnLoad and BindJS.
func TestRegisterExtensions(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()
	x := &testExt{client: &http.Client{}}
	if err := Register(app, Config{TypesDir: t.TempDir(), MigrationsDir: t.TempDir(), Extensions: []any{x}}); err != nil {
		t.Fatal(err)
	}
	code, body := get(t, serve(t, app), "GET", "/ext")
	if code != 200 || body != "ext:embedded pong" {
		t.Fatalf("%d %q", code, body)
	}
	if x.inits.Load() != 1 || x.loads.Load() != 1 || x.calls.Load() < 1 {
		t.Fatalf("inits %d loads %d calls %d", x.inits.Load(), x.loads.Load(), x.calls.Load())
	}
	var client *http.Client
	_ = js.Run(app, func(rt js.Runtime) error { client = rt.Engine().(*VM).HTTPClient; return nil })
	if client != x.client {
		t.Fatal("HTTPClient not used")
	}
}

// A hook file that fails to load is an error (HooksWatch off) naming the file.
func TestRegisterBrokenHook(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"pb_hooks/bad.pb.js": "routerAdd(\n  oops("})
	err := Register(app, Config{HooksDir: filepath.Join(dir, "pb_hooks"), MigrationsDir: t.TempDir(), TypesDir: dir})
	if err == nil || !strings.Contains(err.Error(), "bad.pb.js") {
		t.Fatalf("want an error naming bad.pb.js, got %v", err)
	}
}

// Hook files load before the app opens its database, as in PocketBase: a database call at a hook file's top level
// fails with a hint saying where to do it instead, not a bare nil pointer error.
func TestRegisterDatabaseBeforeBootstrap(t *testing.T) {
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()}) // not bootstrapped, as when pocketone starts
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"pb_hooks/top.pb.mjs": `const users = await $app.findCollectionByNameOrId.async("users")`})
	err := Register(app, Config{HooksDir: filepath.Join(dir, "pb_hooks"), MigrationsDir: t.TempDir(), TypesDir: dir})
	if err == nil || !strings.Contains(err.Error(), "top.pb.mjs") || !strings.Contains(err.Error(), "onBootstrap") {
		t.Fatalf("want an error naming top.pb.mjs with the onBootstrap hint, got %v", err)
	}
}

// Errors point at the real file and line: no error mapping needed.
func TestRegisterErrorPositions(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"pb_hooks/main.pb.js": "function boom() {\n\n  missingFn()\n}\n"})
	if err := Register(app, Config{HooksDir: filepath.Join(dir, "pb_hooks"), MigrationsDir: t.TempDir(), TypesDir: dir}); err != nil {
		t.Fatal(err)
	}
	var stack any
	if err := js.Run(app, func(rt js.Runtime) error {
		r, err := rt.Engine().(*VM).RunString("(() => { try { boom() } catch (e) { return String(e.stack) } })()")
		stack = r.Export()
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if s, _ := stack.(string); !strings.Contains(s, "pb_hooks/main.pb.js:3") {
		t.Fatalf("stack without the file's position: %q", stack)
	}
}
