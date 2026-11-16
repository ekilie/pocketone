package jsvm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// Concurrent requests share one JS scope: top-level variables, objects and modules in pb_hooks are the same live
// values for every request, with no lost updates, including while requests are suspended at an await.

// sharedHooks is one app's pb_hooks: state in one file, routes in others, a module required by two files.
var sharedHooks = map[string]string{
	"pb_hooks/a_state.pb.js": `
		let hits = 0             // every request bumps it
		const seen = new Set()   // every request adds its id
		const waiters = {}       // suspended requests, woken by another request
		const log = []           // start/end events of overlapping async requests
	`,
	"pb_hooks/b_counter.pb.js": `
		routerAdd("GET", "/hit/{id}", (e) => {
			hits++
			seen.add(e.request.pathValue("id"))
			return e.json(200, { hits })
		})
		routerAdd("GET", "/stats", (e) => e.json(200, { hits, seen: seen.size }))
	`,
	"pb_hooks/c_handoff.pb.js": `
		// park this request until another request hands it a value
		routerAdd("GET", "/wait/{id}", async (e) => {
			const id = e.request.pathValue("id")
			const got = await new Promise((resolve) => { waiters[id] = resolve })
			return e.json(200, { id, got, hitsNow: hits })
		})
		routerAdd("GET", "/pending", (e) => e.json(200, { n: Object.keys(waiters).length }))
		routerAdd("GET", "/release/{id}/{value}", (e) => {
			const id = e.request.pathValue("id")
			if (!waiters[id]) return e.json(409, { error: "not waiting" })
			const resolve = waiters[id]
			delete waiters[id]
			resolve(e.request.pathValue("value"))
			return e.json(200, { ok: true })
		})
	`,
	"pb_hooks/d_overlap.pb.js": `
		// read shared state, wait in Go (off the loop), then write: other requests run in between
		routerAdd("GET", "/overlap/{id}", async (e) => {
			const id = e.request.pathValue("id")
			log.push("start " + id)
			await sleep.async(100)
			log.push("end " + id)
			return e.json(200, { id })
		})
		routerAdd("GET", "/log", (e) => e.json(200, { log }))
	`,
	"pb_hooks/lib/cache.js": `module.exports = new Map()`,
	"pb_hooks/e_writer.pb.js": `
		const writeCache = require("./lib/cache.js")
		routerAdd("GET", "/cache/put/{k}/{v}", (e) => {
			writeCache.set(e.request.pathValue("k"), e.request.pathValue("v"))
			return e.json(200, { size: writeCache.size })
		})
	`,
	"pb_hooks/f_reader.pb.js": `
		const readCache = require("./lib/cache.js")
		routerAdd("GET", "/cache/all", (e) => e.json(200, Object.fromEntries(readCache)))
	`,
}

func sharedApp(t *testing.T) http.Handler {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	dir := t.TempDir()
	writeFiles(t, dir, sharedHooks)
	if err := Register(app, Config{HooksDir: filepath.Join(dir, "pb_hooks"), TypesDir: dir}); err != nil {
		t.Fatal(err)
	}
	return serve(t, app)
}

func getJSON(t *testing.T, h http.Handler, url string, out any) int {
	t.Helper()
	code, body := get(t, h, "GET", url)
	if err := json.Unmarshal([]byte(body), out); err != nil {
		t.Fatalf("%s: %d %s", url, code, body)
	}
	return code
}

// 500 requests at once each bump one top-level counter: every request sees a distinct value and none is lost.
func TestSharedCounterUnderConcurrency(t *testing.T) {
	h := sharedApp(t)
	const n = 500
	got := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var r struct{ Hits int }
			if code := getJSON(t, h, fmt.Sprintf("/hit/%d", i), &r); code != 200 {
				t.Errorf("hit %d: %d", i, code)
			}
			got[i] = r.Hits
		}(i)
	}
	wg.Wait()

	sort.Ints(got)
	for i, v := range got {
		if v != i+1 {
			t.Fatalf("hits seen by the requests aren't 1..%d (lost or duplicated update): sorted[%d] = %d", n, i, v)
		}
	}
	var s struct{ Hits, Seen int }
	getJSON(t, h, "/stats", &s)
	if s.Hits != n || s.Seen != n {
		t.Fatalf("stats = %+v, want %d hits and %d ids", s, n, n)
	}
}

// 20 requests park on promises kept in a shared object, all in flight at once; other requests wake each one with a
// value, and each parked request returns exactly the value it was handed (plus the live counter).
func TestSharedStateAcrossSuspendedRequests(t *testing.T) {
	h := sharedApp(t)
	const n = 20
	type result struct {
		ID, Got string
		HitsNow int
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if code := getJSON(t, h, fmt.Sprintf("/wait/w%d", i), &results[i]); code != 200 {
				t.Errorf("wait %d: %d", i, code)
			}
		}(i)
	}

	// all 20 are suspended inside their handlers at the same time
	deadline := time.Now().Add(5 * time.Second)
	for {
		var p struct{ N int }
		getJSON(t, h, "/pending", &p)
		if p.N == n {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d requests are waiting", p.N, n)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// a request in between changes shared state the parked ones will read when they resume
	get(t, h, "GET", "/hit/between")

	for i := 0; i < n; i++ {
		if code, body := get(t, h, "GET", fmt.Sprintf("/release/w%d/value-%d", i, i)); code != 200 {
			t.Fatalf("release %d: %d %s", i, code, body)
		}
	}
	wg.Wait()

	for i, r := range results {
		if r.ID != fmt.Sprintf("w%d", i) || r.Got != fmt.Sprintf("value-%d", i) {
			t.Fatalf("request %d got %+v", i, r)
		}
		if r.HitsNow != 1 {
			t.Fatalf("request %d saw hits=%d after resuming, want the live value 1", i, r.HitsNow)
		}
	}
	var p struct{ N int }
	if getJSON(t, h, "/pending", &p); p.N != 0 {
		t.Fatalf("%d waiters left", p.N)
	}
}

// 20 async requests each log "start", wait 100 ms in Go, then log "end", into one shared array. They overlap (all
// start before any ends, total far under 20 x 100 ms) and every event lands in the same array.
func TestSharedStateWhileRequestsOverlap(t *testing.T) {
	h := sharedApp(t)
	const n = 20
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if code, body := get(t, h, "GET", fmt.Sprintf("/overlap/%d", i)); code != 200 {
				t.Errorf("overlap %d: %d %s", i, code, body)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	var l struct{ Log []string }
	getJSON(t, h, "/log", &l)
	if len(l.Log) != 2*n {
		t.Fatalf("log has %d events, want %d: %v", len(l.Log), 2*n, l.Log)
	}
	starts := 0
	for _, ev := range l.Log[:n] {
		if strings.HasPrefix(ev, "start ") {
			starts++
		}
	}
	if starts != n {
		t.Fatalf("requests didn't overlap: the first %d events aren't all starts: %v", n, l.Log)
	}
	if elapsed > time.Duration(n)*100*time.Millisecond/4 {
		t.Fatalf("took %v: the requests ran one after another", elapsed)
	}
	t.Logf("%d overlapping requests in %v", n, elapsed.Round(time.Millisecond))
}

// A module required by two hook files is one instance: concurrent writes through one file are all visible through
// the other.
func TestSharedModuleAcrossHookFiles(t *testing.T) {
	h := sharedApp(t)
	const n = 100
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if code, body := get(t, h, "GET", fmt.Sprintf("/cache/put/k%d/v%d", i, i)); code != 200 {
				t.Errorf("put %d: %d %s", i, code, body)
			}
		}(i)
	}
	wg.Wait()

	all := map[string]string{}
	getJSON(t, h, "/cache/all", &all)
	if len(all) != n {
		t.Fatalf("reader sees %d keys, want %d", len(all), n)
	}
	for i := 0; i < n; i++ {
		if all[fmt.Sprintf("k%d", i)] != fmt.Sprintf("v%d", i) {
			t.Fatalf("k%d = %q", i, all[fmt.Sprintf("k%d", i)])
		}
	}
}
