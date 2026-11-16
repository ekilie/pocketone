package jsvm

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// require() in pb_hooks: every way of naming a module, one instance per module however many files and requests use
// it, and module-level state shared by concurrent requests.

var importHooks = map[string]string{
	// a module with its own private state, required by two hook files and lazily from inside a handler
	"pb_hooks/lib/counter.js": `
		let n = 0
		module.exports = { inc: () => ++n, get: () => n, loadedAt: Date.now() + Math.random() }
	`,
	// nested relative requires, each resolved from the requiring module's own folder
	"pb_hooks/lib/a.js":          `const b = require("./b"); module.exports = "a>" + b`,
	"pb_hooks/lib/b.js":          `const c = require("../util/c.js"); module.exports = "b>" + c`,
	"pb_hooks/util/c.js":         `exports.toString = () => "c"; exports.name = "c"; module.exports = exports.name`,
	"pb_hooks/lib/settings.json": `{"site": "PocketOne", "limits": {"max": 3}}`,
	// a folder module (index.js)
	"pb_hooks/lib/greeter/index.js": `module.exports = (n) => "hi " + n`,
	// node_modules: package.json "main", and a package that requires another package
	"pb_hooks/node_modules/slugify/package.json": `{"name": "slugify", "main": "src/slug.js"}`,
	"pb_hooks/node_modules/slugify/src/slug.js": `
		const lower = require("lowercase")
		module.exports = (s) => lower(s).replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "")
	`,
	"pb_hooks/node_modules/lowercase/index.js": `module.exports = (s) => String(s).toLowerCase()`,
	// a cycle: x requires y, y requires x and gets x's exports as they were at that point (Node's rule)
	"pb_hooks/lib/x.js": `exports.early = "x-early"; const y = require("./y"); exports.fromY = y.sawX; exports.late = "x-late"`,
	"pb_hooks/lib/y.js": `const x = require("./x"); exports.sawX = Object.keys(x).join(",")`,
	// module metadata
	"pb_hooks/lib/where.js": `module.exports = { file: __filename, dir: __dirname, id: module.id }`,

	"pb_hooks/a_routes.pb.js": `
		const counterA = require("./lib/counter.js")
		const chain = require("./lib/a")
		const settings = require("./lib/settings.json")
		const greet = require("./lib/greeter")
		const slugify = require("slugify")
		const cyc = require("./lib/x")
		const where = require("./lib/where")

		routerAdd("GET", "/a/inc", (e) => e.json(200, { n: counterA.inc() }))
		routerAdd("GET", "/kinds", (e) => e.json(200, {
			chain,
			site: settings.site,
			max: settings.limits.max,
			greet: greet("ben"),
			slug: slugify("  Hello, PocketOne World! "),
			cycle: cyc,
			where,
		}))
		routerAdd("GET", "/missing", (e) => {
			try { require("./lib/nope") } catch (err) { return e.json(200, { error: String(err) }) }
			return e.json(500, { error: "no error" })
		})
	`,
	"pb_hooks/b_routes.pb.js": `
		const counterB = require("./lib/counter")    // same file, no extension: the same instance
		routerAdd("GET", "/b/inc", (e) => e.json(200, { n: counterB.inc() }))
		routerAdd("GET", "/count", (e) => {
			const lazy = require("./lib/counter.js")  // required at request time: still the same instance
			return e.json(200, { n: lazy.get(), same: lazy === counterB, loadedAt: lazy.loadedAt })
		})
	`,
}

func importsApp(t *testing.T) http.Handler {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	dir := t.TempDir()
	writeFiles(t, dir, importHooks)
	if err := Register(app, Config{HooksDir: filepath.Join(dir, "pb_hooks"), TypesDir: dir}); err != nil {
		t.Fatal(err)
	}
	return serve(t, app)
}

// Every way of naming a module resolves and loads.
func TestImportKinds(t *testing.T) {
	h := importsApp(t)
	var r struct {
		Chain, Site, Greet, Slug string
		Max                      int
		Cycle                    struct{ Early, FromY, Late string }
		Where                    struct{ File, Dir, ID string }
	}
	if code := getJSON(t, h, "/kinds", &r); code != 200 {
		t.Fatalf("/kinds: %d", code)
	}
	checks := []struct{ what, got, want string }{
		{"nested relative requires", r.Chain, "a>b>c"},
		{"json module", r.Site, "PocketOne"},
		{"json nested value", fmt.Sprint(r.Max), "3"},
		{"folder index.js", r.Greet, "hi ben"},
		{"node_modules package requiring a package", r.Slug, "hello-pocketone-world"},
		{"cycle: x before requiring y", r.Cycle.Early, "x-early"},
		{"cycle: y saw x's partial exports", r.Cycle.FromY, "early"},
		{"cycle: x after y", r.Cycle.Late, "x-late"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.what, c.got, c.want)
		}
	}
	if !strings.HasSuffix(r.Where.File, "/lib/where.js") || !strings.HasSuffix(r.Where.Dir, "/lib") || r.Where.ID != r.Where.File {
		t.Errorf("__filename/__dirname/module.id: %+v", r.Where)
	}

	var m struct{ Error string }
	if getJSON(t, h, "/missing", &m); !strings.Contains(m.Error, "Cannot find module './lib/nope'") {
		t.Errorf("missing module error: %q", m.Error)
	}
}

// A module is loaded once: two hook files and a request-time require all get the same instance, and its private
// state is shared by 400 concurrent requests through both files with no lost updates.
func TestImportedModuleSharedByConcurrentRequests(t *testing.T) {
	h := importsApp(t)
	var before struct {
		LoadedAt float64
	}
	getJSON(t, h, "/count", &before)

	const per = 200
	var wg sync.WaitGroup
	seen := make(chan int, 2*per)
	for i := 0; i < per; i++ {
		for _, path := range []string{"/a/inc", "/b/inc"} {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				var r struct{ N int }
				if code := getJSON(t, h, path, &r); code != 200 {
					t.Errorf("%s: %d", path, code)
				}
				seen <- r.N
			}(path)
		}
	}
	wg.Wait()
	close(seen)

	distinct := map[int]bool{}
	for n := range seen {
		if distinct[n] {
			t.Fatalf("two requests got the same count %d: the module's state isn't shared safely", n)
		}
		distinct[n] = true
	}
	var after struct {
		N        int
		Same     bool
		LoadedAt float64
	}
	getJSON(t, h, "/count", &after)
	if after.N != 2*per || len(distinct) != 2*per {
		t.Fatalf("count = %d (%d distinct), want %d", after.N, len(distinct), 2*per)
	}
	if !after.Same {
		t.Fatal("request-time require returned a different instance than the hook file's")
	}
	if after.LoadedAt != before.LoadedAt {
		t.Fatal("the module was loaded more than once")
	}
}
