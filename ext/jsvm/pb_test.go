//go:build cgo

package jsvm

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// pbApp loads hooks into a fresh VM bound to a test app and returns an HTTP handler for its router.
func pbApp(t testing.TB, cfg VMConfig, hooks string) (*tests.TestApp, *VM, http.Handler) {
	return pbAppOpts(t, cfg, hooks, true)
}

// pbAppNoClose leaves the app and VM open (for tests that may leave the loop stuck).
func pbAppNoClose(t testing.TB, cfg VMConfig, hooks string) (*tests.TestApp, *VM, http.Handler) {
	return pbAppOpts(t, cfg, hooks, false)
}

func pbAppOpts(t testing.TB, cfg VMConfig, hooks string, cleanup bool) (*tests.TestApp, *VM, http.Handler) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	var vm *VM
	if cleanup {
		t.Cleanup(app.Cleanup)
		vm = newVM(t, cfg)
	} else {
		vm, err = New(cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	vm.Log = func(a ...any) { t.Log(a...) }
	if err := BindApp(vm, app); err != nil {
		t.Fatal(err)
	}
	if err := vm.Eval(hooks, "main.pb.js"); err != nil {
		t.Fatal(err)
	}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	se := &core.ServeEvent{App: app, Router: router}
	var mux http.Handler
	if err := app.OnServe().Trigger(se, func(e *core.ServeEvent) error {
		m, err := e.Router.BuildMux()
		mux = m
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return app, vm, mux
}

func get(t testing.TB, h http.Handler, method, url string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, url, nil))
	return rec.Code, rec.Body.String()
}

const hooksJS = `
// Top-level helpers and state: visible in every handler (jsvm can't do this).
const prefix = "hello "
let creates = 0
function greet(name) { return prefix + name }

onRecordCreate((e) => {
	creates++
	e.record.set("title", e.record.get("title") + " [hooked " + creates + "]")
	return e.next()
}, "demo2")

routerAdd("GET", "/greet/{name}", (e) => {
	return e.json(200, { msg: greet(e.request.pathValue("name")), creates })
})

// sync compat: today's API, Go calls return values directly
routerAdd("POST", "/sync-create", (e) => {
	const col = $app.findCollectionByNameOrId("demo2")
	const r = new Record(col)
	r.set("title", "sync")
	$app.save(r) // fires onRecordCreate inline on the loop (re-entrant)
	return e.json(200, { title: r.get("title") })
})

// async: the save runs off the loop; the hook comes back to the loop while this handler is suspended
routerAdd("POST", "/async-create", async (e) => {
	const col = await $app.findCollectionByNameOrId.async("demo2")
	const r = new Record(col)
	r.set("title", "async")
	await e.app.save.async(r)
	return e.json(200, { title: r.get("title") })
})

routerAdd("GET", "/list", (e) => {
	const rows = arrayOf(new Record())
	$app.recordQuery("demo2").orderBy("title").limit(3).all(rows)
	const titles = []
	for (const r of rows) titles.push(r.get("title"))
	return e.json(200, { n: rows.length, titles })
})

routerAdd("GET", "/slow-async", async (e) => {
	await sleep.async(100)
	return e.string(200, "ok")
})

routerAdd("GET", "/slow-sync", (e) => {
	sleep(100)
	return e.string(200, "ok")
})

routerAdd("POST", "/tx", (e) => {
	$app.runInTransaction((txApp) => {
		const r = new Record(txApp.findCollectionByNameOrId("demo2"))
		r.set("title", "tx")
		txApp.save(r)
	})
	return e.string(200, "ok")
})
`

func TestPBGlobalsInHandlers(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{}, hooksJS)
	code, body := get(t, h, "GET", "/greet/ben")
	if code != 200 || !strings.Contains(body, `"msg":"hello ben"`) {
		t.Fatal(code, body)
	}
}

func TestPBSyncReentrant(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{}, hooksJS)
	code, body := get(t, h, "POST", "/sync-create")
	if code != 200 || !strings.Contains(body, `sync [hooked 1]`) {
		t.Fatal(code, body)
	}
	_, body = get(t, h, "GET", "/greet/x")
	if !strings.Contains(body, `"creates":1`) {
		t.Fatal("shared state:", body)
	}
}

func TestPBAsyncReentrant(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{}, hooksJS)
	code, body := get(t, h, "POST", "/async-create")
	if code != 200 || !strings.Contains(body, `async [hooked 1]`) {
		t.Fatal(code, body)
	}
}

func TestPBArrayOfRecordQuery(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{}, hooksJS)
	code, body := get(t, h, "GET", "/list")
	if code != 200 || !strings.Contains(body, `"n":3`) {
		t.Fatal(code, body)
	}
}

func TestPBTransactionSync(t *testing.T) {
	app, _, h := pbApp(t, VMConfig{}, hooksJS)
	code, body := get(t, h, "POST", "/tx")
	if code != 200 {
		t.Fatal(code, body)
	}
	if _, err := app.FindFirstRecordByFilter("demo2", `title="tx [hooked 1]"`); err != nil {
		t.Fatal(err)
	}
}

// Async handlers overlap on one VM; sync ones queue behind each other.
func TestPBConcurrency(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{}, hooksJS)
	run := func(path string) time.Duration {
		start := time.Now()
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if code, body := get(t, h, "GET", path); code != 200 {
					t.Error(code, body)
				}
			}()
		}
		wg.Wait()
		return time.Since(start)
	}
	a := run("/slow-async")
	s := run("/slow-sync")
	t.Logf("10 x 100ms: async %v, sync %v", a, s)
	if a > 400*time.Millisecond {
		t.Fatalf("async should overlap: %v", a)
	}
	if s < 900*time.Millisecond {
		t.Fatalf("sync should serialize: %v", s)
	}
}

func pumpCfg() VMConfig { return VMConfig{PumpSyncCalls: true, Blocking: PBBlocking} }

// With pumping, all the compat tests still pass and sync handlers overlap at Go calls.
func TestPBPumped(t *testing.T) {
	for _, path := range []struct{ m, p, want string }{
		{"POST", "/sync-create", "sync [hooked 1]"},
		{"POST", "/async-create", "async [hooked 1]"},
		{"GET", "/list", `"n":3`},
		{"POST", "/tx", "ok"},
	} {
		_, _, h := pbApp(t, pumpCfg(), hooksJS)
		code, body := get(t, h, path.m, path.p)
		if code != 200 || !strings.Contains(body, path.want) {
			t.Fatal(path.p, code, body)
		}
	}
}

func TestPBPumpedConcurrency(t *testing.T) {
	for _, n := range []int{10, 40} {
		for _, depth := range []int{16, 64} {
			cfg := pumpCfg()
			cfg.MaxPumpDepth = depth
			_, vm, h := pbApp(t, cfg, hooksJS)
			start := time.Now()
			var wg sync.WaitGroup
			fails := 0
			var mu sync.Mutex
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if code, body := get(t, h, "GET", "/slow-sync"); code != 200 {
						mu.Lock()
						fails++
						if fails == 1 {
							t.Log(code, body)
						}
						mu.Unlock()
					}
				}()
			}
			wg.Wait()
			t.Logf("%d sync x 100ms, depth %d: %v, max depth %d, fails %d", n, depth, time.Since(start), vm.statMaxPump.Load(), fails)
		}
	}
}
