package pocker

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A pb_hooks edit restarts a production app the way PocketBase's watcher does: the running app stops first, then a
// fresh one starts, so two apps never have the folder open. A bad edit leaves the instance down (502 with the error)
// until a fix, and the next request after the fix starts it with the new code.
func TestHooksEditStopsThenStarts(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "pb_hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHook(t, d, "a1")
	a := Instance{ID: "a", Hosts: []string{"a.test"}, Dir: d}
	m := newTestManager(t, false, a)
	defer m.Shutdown()
	events := make(chan Event, 16)
	m.onEvent = func(e Event) { events <- e }

	if got := get(m, "a.test"); got != "a1" {
		t.Fatalf("before: %q", got)
	}
	m.mu.RLock()
	old := m.apps["a"].app
	m.mu.RUnlock()

	writeHook(t, d, "a2")
	waitEvent(t, events, EventRestarted, "pb_hooks changed")
	if old.app.IsBootstrapped() {
		t.Fatal("the old app was still running when the restart was reported")
	}
	waitEvent(t, events, EventStarted, "")
	if got := get(m, "a.test"); got != "a2" {
		t.Fatalf("after the edit: %q", got)
	}

	if err := os.WriteFile(filepath.Join(d, "pb_hooks", "main.pb.js"), []byte("this is not javascript ("), 0o644); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, EventFailed, "main.pb.js")
	w := httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://a.test/v", nil))
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "main.pb.js") {
		t.Fatalf("a broken production edit: %d %q, want 502 naming the file", w.Code, w.Body.String())
	}
	log, _ := os.ReadFile(filepath.Join(d, "logs", "exec.log"))
	if !strings.Contains(string(log), "Instance failed to start") {
		t.Errorf("instance log should have the start error:\n%s", log)
	}

	writeHook(t, d, "a3")
	if got := get(m, "a.test"); got != "a3" {
		t.Fatalf("after the fix: %q", got)
	}
}

// In dev mode a hook file that fails to load is skipped: the app starts and the other files' routes work.
func TestDevModeSkipsBrokenHookFile(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "pb_hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHook(t, d, "ok")
	if err := os.WriteFile(filepath.Join(d, "pb_hooks", "broken.pb.js"), []byte("this is not javascript ("), 0o644); err != nil {
		t.Fatal(err)
	}

	dev := Instance{ID: "dev", Hosts: []string{"dev.test"}, Dir: d, Dev: true}
	m := newTestManager(t, false, dev)
	defer m.Shutdown()
	if got := get(m, "dev.test"); got != "ok" {
		t.Fatalf("dev app should start without the broken file and serve the good one, got %q", got)
	}
	log, _ := os.ReadFile(filepath.Join(d, "logs", "exec.log"))
	if !strings.Contains(string(log), "pb_hooks/broken.pb.js failed to load; skipped it (dev mode)") {
		t.Errorf("instance log should name the skipped file:\n%s", log)
	}

	prod := Instance{ID: "prod", Hosts: []string{"prod.test"}, Dir: d}
	m2 := newTestManager(t, false, prod)
	defer m2.Shutdown()
	if _, err := m2.app(prod); err == nil {
		t.Fatal("a production app with a broken hook file should fail to start")
	}
}

func waitEvent(t *testing.T, events <-chan Event, kind EventKind, reason string) {
	t.Helper()
	timeout := time.After(15 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Kind == kind && strings.Contains(e.Reason, reason) {
				return
			}
		case <-timeout:
			t.Fatalf("no %s event with %q within 15 s", kind, reason)
		}
	}
}
