package main

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pb_hooks on one app, live: a script (.pb.js), an ES module (.pb.mjs) and TypeScript (.pb.ts) side by side in the
// app's one VM, with top-level state that lasts across requests. The hooks watcher restarts the process on an edit
// and the new code serves; a file that fails to load is logged with its position and skipped while the rest serves,
// and serves again once fixed. With --hooksWatch=false the same broken file stops startup.
func TestE2EHooksReload(t *testing.T) {
	s := startSite(t, siteOptions{Files: map[string]string{
		"pb_hooks/a.pb.js":  hookA("v1"),
		"pb_hooks/b.pb.mjs": hookB,
		"pb_hooks/c.pb.ts":  hookC,
	}})

	step(t, "three formats, one VM")
	wantBody(t, s, "/a", "a v1")
	wantBody(t, s, "/b", "b sees: shared from a")
	wantBody(t, s, "/c", "c: 42")
	wantBody(t, s, "/hits", "1")
	wantBody(t, s, "/hits", "2")

	step(t, "a good edit: the process restarts into the new code")
	write(t, filepath.Join(s.Dir, "pb_hooks", "a.pb.js"), hookA("v2"))
	s.waitRestarted("edit a.pb.js", 20*time.Second)
	wantBody(t, s, "/a", "a v2")
	wantBody(t, s, "/hits", "1") // a new process: state starts over

	step(t, "a broken edit: logged with its position, skipped, the rest serves")
	write(t, filepath.Join(s.Dir, "pb_hooks", "b.pb.mjs"), "import { x } from './nowhere.mjs'\n\nrouterAdd('GET', '/b', (e) => e.string(200, 'never'))\n")
	s.waitRestarted("break b.pb.mjs", 20*time.Second)
	if log := s.Log.String(); !strings.Contains(log, "failed to execute b.pb.mjs") || !strings.Contains(log, "nowhere.mjs") {
		t.Fatalf("no load error for b.pb.mjs in the log:\n%s", tail(log, 40))
	}
	wantBody(t, s, "/a", "a v2")
	wantBody(t, s, "/c", "c: 42")
	if code, b := s.raw(http.MethodGet, "/b", "", nil); code != 404 {
		t.Fatalf("GET /b with b.pb.mjs broken: %d %s, want 404", code, b)
	}

	step(t, "fixed: it serves again")
	write(t, filepath.Join(s.Dir, "pb_hooks", "b.pb.mjs"), hookB)
	s.waitRestarted("fix b.pb.mjs", 20*time.Second)
	wantBody(t, s, "/b", "b sees: shared from a")

	step(t, "--hooksWatch=false: a broken file stops startup, naming it")
	s.stop()
	write(t, filepath.Join(s.Dir, "pb_hooks", "c.pb.ts"), "const n: number = 1\nthrow new Error('c is broken')\n")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := runBin(ctx, pocketoneBin(t), s.Dir, s.Env, "serve", "--dir=pb_data", "--http="+freeAddr(t), "--hooksWatch=false").CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("serve with a broken hook kept running instead of stopping:\n%s", tail(string(out), 30))
	}
	if err == nil || !strings.Contains(string(out), "c.pb.ts") || !strings.Contains(string(out), "c is broken") {
		t.Fatalf("serve --hooksWatch=false with a broken c.pb.ts: %v\n%s", err, tail(string(out), 30))
	}
	if !strings.Contains(string(out), "c.pb.ts:2:") {
		t.Fatalf("the startup error doesn't name the line (c.pb.ts:2:):\n%s", tail(string(out), 30))
	}
}

// hookA is a script: its top-level names are the app's globals.
func hookA(version string) string {
	return `let hits = 0
function shared() { return "shared from a" }

routerAdd("GET", "/a", (e) => e.string(200, "a ` + version + `"))
routerAdd("GET", "/hits", (e) => e.string(200, String(++hits)))
`
}

// hookB is an ES module that calls a script's global at request time.
const hookB = `const label = "b sees: "

routerAdd("GET", "/b", (e) => e.string(200, label + globalThis.shared()))
`

// hookC is TypeScript, types stripped on load.
const hookC = `type Answer = { value: number }
const answer: Answer = { value: 6 * 7 }

routerAdd("GET", "/c", (e) => e.string(200, "c: " + answer.value))
`

func wantBody(t *testing.T, s *site, path, want string) {
	t.Helper()
	code, b := s.raw(http.MethodGet, path, "", nil)
	if code != 200 || string(b) != want {
		t.Fatalf("GET %s: %d %q, want %q\n%s", path, code, b, want, tail(s.Log.String(), 30))
	}
}
