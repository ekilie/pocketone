package jsvm

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	pbjsvm "ganigeorgiev.com/pocketbase/pocketbase/plugins/jsvm"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// ---- pure engine: same scripts on goja and on QuickJS ----

var engineCases = []struct{ name, js string }{
	{"plain objects", `let s = 0; for (let i = 0; i < 200000; i++) { const o = { a: i, b: "x", c: [i] }; s += o.a + o.c[0] } s`},
	{"closures", `let fs = []; for (let i = 0; i < 100000; i++) fs.push(() => i); let s = 0; for (const f of fs) s += f(); s`},
	{"binary tree", `function mk(d) { return d === 0 ? {l: null, r: null} : {l: mk(d - 1), r: mk(d - 1)} } function cnt(n) { return n.l ? 1 + cnt(n.l) + cnt(n.r) : 1 } let s = 0; for (let i = 0; i < 20; i++) s += cnt(mk(12)); s`},
	{"churn", `let keep = []; for (let i = 0; i < 300000; i++) { keep[i % 1000] = { i, s: "v" + i } } keep.length`},
	{"Map", `const m = new Map(); for (let i = 0; i < 100000; i++) m.set("k" + i, i); let s = 0; for (const [k, v] of m) s += v; s`},
	{"flat data", `const a = new Array(200000); for (let i = 0; i < a.length; i++) a[i] = i * 2; let s = 0; for (let i = 0; i < a.length; i++) s += a[i]; s`},
	{"string build", `let p = []; for (let i = 0; i < 100000; i++) p.push("item" + i); p.join(",").length`},
	{"json", `const o = []; for (let i = 0; i < 20000; i++) o.push({ id: i, name: "n" + i, tags: ["a", "b"] }); JSON.parse(JSON.stringify(o)).length`},
}

func BenchmarkEngine(b *testing.B) {
	for _, c := range engineCases {
		b.Run("goja/"+c.name, func(b *testing.B) {
			pr := goja.MustCompile("bench.js", c.js, false)
			for i := 0; i < b.N; i++ {
				rt := goja.New()
				if _, err := rt.RunProgram(pr); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("quickjs/"+c.name, func(b *testing.B) {
			vm := newVM(b, VMConfig{MemoryLimit: 512 << 20, TaskBudget: time.Minute})
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := vm.Eval("{"+c.js+"}", "bench.js"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ---- Go interop: reading fields and calling methods on a Go object ----

func BenchmarkInterop(b *testing.B) {
	const loop = `let s = 0; for (let i = 0; i < 10000; i++) { s += post.title.length; s += post.upper().length }`
	b.Run("goja", func(b *testing.B) {
		rt := goja.New()
		rt.SetFieldNameMapper(pbjsvm.FieldMapper{})
		rt.Set("post", &Post{Title: "hello"})
		pr := goja.MustCompile("b.js", "{"+loop+"}", false)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := rt.RunProgram(pr); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("quickjs", func(b *testing.B) {
		vm := newVM(b, VMConfig{TaskBudget: time.Minute})
		vm.Set("post", &Post{Title: "hello"})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := vm.Eval("{"+loop+"}", "b.js"); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ---- whole routes: plugins/jsvm (goja pool) vs one QuickJS VM ----

const benchHooks = `
routerAdd("GET", "/b/find/{id}", (e) => {
	const r = $app.findRecordById("demo2", e.request.pathValue("id"))
	return e.json(200, { t: r.get("title") })
})
routerAdd("GET", "/b/cpu", (e) => {
	let s = 0
	for (let i = 0; i < 20000; i++) { const o = { i, k: "x" + i }; s += o.i + o.k.length }
	return e.json(200, { s })
})
routerAdd("GET", "/b/interop/{id}", (e) => {
	const r = $app.findRecordById("demo2", e.request.pathValue("id"))
	let n = 0
	for (let i = 0; i < 500; i++) n += r.get("title").length
	return e.json(200, { n })
})
`

func setupRouter(t testing.TB, app core.App) http.Handler {
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	var mux http.Handler
	if err := app.OnServe().Trigger(&core.ServeEvent{App: app, Router: router}, func(e *core.ServeEvent) error {
		m, err := e.Router.BuildMux()
		mux = m
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return mux
}

type loadResult struct {
	rps           float64
	p50, p99, max time.Duration
	errs          int
}

func load(h http.Handler, path string, clients, per int) loadResult {
	var mu sync.Mutex
	var lat []time.Duration
	errs := 0
	start := time.Now()
	var wg sync.WaitGroup
	for c := 0; c < clients; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < per; i++ {
				s := time.Now()
				code, _ := getq(h, path)
				d := time.Since(s)
				mu.Lock()
				lat = append(lat, d)
				if code != 200 {
					errs++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	el := time.Since(start)
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	return loadResult{rps: float64(len(lat)) / el.Seconds(), p50: lat[len(lat)/2], p99: lat[len(lat)*99/100], max: lat[len(lat)-1], errs: errs}
}

func TestRouteBench(t *testing.T) {
	if os.Getenv("ASYNCVM_BENCH") == "" {
		t.Skip("set ASYNCVM_BENCH=1")
	}
	setups := []struct {
		name string
		mk   func(t testing.TB) (http.Handler, string)
	}{
		{"jsvm goja pool(16)", func(t testing.TB) (http.Handler, string) {
			app, _ := tests.NewTestApp()
			t.Cleanup(app.Cleanup)
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "main.pb.js"), []byte(benchHooks), 0o644)
			if err := pbjsvm.Register(app, pbjsvm.Config{HooksDir: dir, MigrationsDir: t.TempDir(), HooksPoolSize: 16, TypesDir: t.TempDir()}); err != nil {
				t.Fatal(err)
			}
			return setupRouter(t, app), firstID(t, app)
		}},
		{"asyncvm sync, no pump", func(t testing.TB) (http.Handler, string) {
			app, _, h := pbApp(t, VMConfig{}, benchHooks)
			return h, firstID(t, app)
		}},
		{"asyncvm sync, pumped", func(t testing.TB) (http.Handler, string) {
			app, _, h := pbApp(t, VMConfig{PumpSyncCalls: true, Blocking: PBBlocking, MaxPumpDepth: 64}, benchHooks)
			return h, firstID(t, app)
		}},
	}
	var out []string
	for _, s := range setups {
		h, id := s.mk(t)
		for _, path := range []string{"/b/find/" + id, "/b/cpu", "/b/interop/" + id} {
			load(h, path, 4, 20) // warm up
			r := load(h, path, 16, 100)
			out = append(out, fmt.Sprintf("%-24s %-18s %8.0f req/s  p50 %-10v p99 %-10v max %-10v errs %d", s.name, strings.SplitN(path[3:], "/", 2)[0], r.rps, r.p50.Round(time.Microsecond), r.p99.Round(time.Microsecond), r.max.Round(time.Microsecond), r.errs))
		}
	}
	t.Log("\n" + strings.Join(out, "\n"))
}

func firstID(t testing.TB, app core.App) string {
	rs, err := app.FindAllRecords("demo2")
	if err != nil || len(rs) == 0 {
		t.Fatal(err)
	}
	return rs[0].Id
}

func getq(h http.Handler, url string) (int, string) {
	rec := newRec()
	h.ServeHTTP(rec, mkReq(url))
	return rec.Code, ""
}
