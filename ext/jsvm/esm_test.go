package jsvm

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// ES modules in pb_hooks and pb_migrations: *.pb.mjs hook files and .mjs modules use import/export; CommonJS
// (.cjs, plain .js, .json, node_modules packages) imports as the same instance require() returns.

var esmHooks = map[string]string{
	// shared state lives in a module, not in globals
	"pb_hooks/lib/store.mjs": `
		globalThis.storeLoads = (globalThis.storeLoads || 0) + 1
		export const state = { hits: 0 }
		export function bump() { return ++state.hits }
	`,
	"pb_hooks/lib/format.mjs": `
		import { state } from "./store.mjs"
		export default (n) => "#" + n + " of " + state.hits
	`,
	"pb_hooks/lib/legacy.cjs": `
		let n = 0
		module.exports = { inc: () => ++n, name: "legacy" }
	`,
	"pb_hooks/lib/data.json": `{"site": "PocketOne"}`,
	// a dual package: "exports" picks the ES build for import and the CommonJS build for require
	"pb_hooks/node_modules/dual/package.json": `{"name": "dual", "type": "module",
		"exports": {".": {"import": "./dist/index.js", "require": "./dist/index.cjs"}}}`,
	"pb_hooks/node_modules/dual/dist/index.js":  `export const kind = "esm"`,
	"pb_hooks/node_modules/dual/dist/index.cjs": `module.exports = { kind: "cjs" }`,
	// a plain CommonJS package
	"pb_hooks/node_modules/greet/index.js": `exports.hello = (n) => "hello " + n`,

	"pb_hooks/a.pb.mjs": `
		import { bump, state } from "./lib/store.mjs"
		import fmt from "./lib/format.mjs"
		import legacy, { inc } from "./lib/legacy.cjs"
		import data from "./lib/data.json"
		import { kind } from "dual"
		import { hello } from "greet"

		const secret = "private to a.pb.mjs"
		const booted = await sleep.async(10) ?? "top-level await done"

		routerAdd("GET", "/esm/bump", (e) => {
			const n = bump()
			return e.json(200, { n, text: fmt(n) })
		})
		routerAdd("GET", "/esm/inc", (e) => e.json(200, { n: inc() }))
		routerAdd("GET", "/esm/kinds", (e) => e.json(200, {
			legacyName: legacy.name,
			namedIsDefault: inc === legacy.inc,
			site: data.site,
			kind,
			hello: hello("ben"),
			url: import.meta.url,
			booted,
			secret,
			loads: globalThis.storeLoads,
		}))
		routerAdd("GET", "/esm/lazy", async (e) => {
			const m = await import("./lib/store.mjs")
			return e.json(200, { same: m.state === state })
		})
	`,
	"pb_hooks/b.pb.mjs": `
		import { bump } from "./lib/store.mjs"
		routerAdd("GET", "/esm2/bump", (e) => e.json(200, { n: bump() }))
		routerAdd("GET", "/esm2/private", (e) => e.json(200, { secret: typeof secret }))
	`,
	"pb_hooks/c.pb.cjs": `
		const legacyC = require("./lib/legacy.cjs")
		routerAdd("GET", "/cjs/inc", (e) => e.json(200, { n: legacyC.inc() }))
		routerAdd("GET", "/cjs/pkg", (e) => e.json(200, { kind: require("dual").kind }))
		routerAdd("GET", "/cjs/require-esm", (e) => {
			try { require("./lib/store.mjs") } catch (err) { return e.json(200, { error: String(err) }) }
			return e.json(500, {})
		})
		// a script reaches ES modules with import()
		routerAdd("GET", "/cjs/bump", async (e) => {
			const { bump } = await import("./lib/store.mjs")
			return e.json(200, { n: bump() })
		})
	`,
}

func esmApp(t *testing.T, extra map[string]string) (core.App, http.Handler, error) {
	t.Helper()
	saved := core.AppMigrations
	t.Cleanup(func() { core.AppMigrations = saved })
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	dir := t.TempDir()
	files := map[string]string{}
	for k, v := range esmHooks {
		files[k] = v
	}
	for k, v := range extra {
		files[k] = v
	}
	writeFiles(t, dir, files)
	err = Register(app, Config{HooksDir: filepath.Join(dir, "pb_hooks"), MigrationsDir: filepath.Join(dir, "pb_migrations"), TypesDir: dir})
	if err != nil {
		return app, nil, err
	}
	return app, serve(t, app), nil
}

// Every kind of import works from a .pb.mjs hook file, top-level await finishes before the app serves, and a hook
// module's top level is private.
func TestESMImportKinds(t *testing.T) {
	_, h, err := esmApp(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		LegacyName, Site, Kind, Hello, URL, Booted, Secret string
		NamedIsDefault                                     bool
		Loads                                              int
	}
	if code := getJSON(t, h, "/esm/kinds", &r); code != 200 {
		t.Fatalf("/esm/kinds: %d", code)
	}
	checks := []struct{ what, got, want string }{
		{"CommonJS default export", r.LegacyName, "legacy"},
		{"CommonJS named export is the same function", fmt.Sprint(r.NamedIsDefault), "true"},
		{"JSON default export", r.Site, "PocketOne"},
		{"dual package, import condition", r.Kind, "esm"},
		{"CommonJS package from node_modules", r.Hello, "hello ben"},
		{"top-level await", r.Booted, "top-level await done"},
		{"module's own top level", r.Secret, "private to a.pb.mjs"},
		{"a module imported by three files loads once", fmt.Sprint(r.Loads), "1"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.what, c.got, c.want)
		}
	}
	if !strings.HasPrefix(r.URL, "file://") {
		t.Errorf("import.meta.url = %q", r.URL)
	}

	var p struct{ Secret string }
	if getJSON(t, h, "/esm2/private", &p); p.Secret != "undefined" {
		t.Errorf("another module sees a.pb.mjs's top-level const: typeof secret = %q", p.Secret)
	}
	var k struct{ Kind string }
	if getJSON(t, h, "/cjs/pkg", &k); k.Kind != "cjs" {
		t.Errorf("dual package, require condition: got %q", k.Kind)
	}
	var e struct{ Error string }
	if getJSON(t, h, "/cjs/require-esm", &e); !strings.Contains(e.Error, "use import") {
		t.Errorf("require() of an ES module: %q", e.Error)
	}
}

// One module instance is shared by every file and request: 500 concurrent requests bump the same exported state
// through two .pb.mjs files and a script's import(), and a CommonJS counter is shared by import and require.
func TestESMStateSharedByConcurrentRequests(t *testing.T) {
	_, h, err := esmApp(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := func(paths []string, per int) map[int]bool {
		seen := make(chan int, per*len(paths))
		var wg sync.WaitGroup
		for i := 0; i < per; i++ {
			for _, p := range paths {
				wg.Add(1)
				go func(p string) {
					defer wg.Done()
					var r struct{ N int }
					if code := getJSON(t, h, p, &r); code != 200 {
						t.Errorf("%s: %d", p, code)
					}
					seen <- r.N
				}(p)
			}
		}
		wg.Wait()
		close(seen)
		distinct := map[int]bool{}
		for n := range seen {
			if distinct[n] {
				t.Fatalf("%v: two requests got %d (state not shared safely)", paths, n)
			}
			distinct[n] = true
		}
		return distinct
	}

	if got := run([]string{"/esm/bump", "/esm2/bump"}, 200); len(got) != 400 {
		t.Fatalf("got %d distinct counts, want 400", len(got))
	}
	if got := run([]string{"/cjs/bump"}, 100); len(got) != 100 {
		t.Fatalf("got %d distinct counts from import(), want 100", len(got))
	}
	var b struct {
		N    int
		Text string
	}
	getJSON(t, h, "/esm/bump", &b)
	if b.N != 501 || b.Text != "#501 of 501" {
		t.Fatalf("after 500 bumps: %+v", b)
	}
	var lazy struct{ Same bool }
	if getJSON(t, h, "/esm/lazy", &lazy); !lazy.Same {
		t.Fatal("import() returned a different instance than the static import")
	}

	if got := run([]string{"/esm/inc", "/cjs/inc"}, 150); len(got) != 300 {
		t.Fatalf("CommonJS counter through import and require: %d distinct, want 300", len(got))
	}
}

// Errors name the file: a missing import, a syntax error in a module, a throw during top-level await.
func TestESMErrors(t *testing.T) {
	cases := []struct {
		name, file, src, want string
	}{
		{"missing import", "pb_hooks/z.pb.mjs", `import { x } from "./lib/nope.mjs"`, "Cannot find module './lib/nope.mjs'"},
		{"syntax error", "pb_hooks/z.pb.mjs", "export const a = 1\nexport const = 2", "z.pb.mjs:2"},
		{"top-level await rejects", "pb_hooks/z.pb.mjs", `await Promise.reject(new Error("boot failed"))`, "boot failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := esmApp(t, map[string]string{c.file: c.src})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error containing %q, got %v", c.want, err)
			}
		})
	}
}

// Migrations can be ES modules: each has its own scope (the same top-level name in two files is fine), they import
// helpers, and top-level await works.
func TestESMMigrations(t *testing.T) {
	app, _, err := esmApp(t, map[string]string{
		"pb_migrations/lib/names.mjs": `export const table = "esm_migrated"`,
		"pb_migrations/1700000000_create.mjs": `
			import { table } from "./lib/names.mjs"
			const label = "first"
			await sleep.async(5)
			migrate((app) => {
				app.save(new Collection({ name: table, type: "base", fields: [{ name: "title", type: "text" }] }))
			}, (app) => {
				app.delete(app.findCollectionByNameOrId(table))
			})
		`,
		"pb_migrations/1700000001_second.mjs": `const label = "second"; migrate((app) => {})`,
		"pb_migrations/1700000002_old.js":     `const label = "plain js"; migrate((app) => {})`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.RunAppMigrations(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindCollectionByNameOrId("esm_migrated"); err != nil {
		t.Fatalf("the .mjs migration didn't run: %v", err)
	}
}
