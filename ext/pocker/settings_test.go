package pocker

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"pocketone.dev/ext/contracts/instance/source/v1/sourcetest"
)

// A settings change from instance.source/v1 stops the running app before the next one starts: by the time the
// instance serves its new settings, the old app is no longer bootstrapped.
func TestSettingsChangeStopsBeforeStarting(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "pb_hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	hook := `routerAdd("GET", "/v", (e) => e.string(200, $os.getenv("V")))`
	if err := os.WriteFile(filepath.Join(d, "pb_hooks", "main.pb.js"), []byte(hook), 0o644); err != nil {
		t.Fatal(err)
	}
	inst := Instance{ID: "a", Hosts: []string{"a.test"}, Dir: d, Secrets: map[string]string{"V": "one"}}
	src, scope := sourcetest.Scope(inst)
	m := NewManager(scope, ManagerOptions{Standard: testStandard, PBRoot: t.TempDir(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	runManager(t, m)
	defer m.Shutdown()

	if got := get(m, "a.test"); got != "one" {
		t.Fatalf("before: %q", got)
	}
	m.mu.RLock()
	old := m.apps["a"].app
	m.mu.RUnlock()

	inst.Secrets = map[string]string{"V": "two"}
	src.Set(inst)
	waitUntil(t, "the instance to serve its new secret", func() bool { return get(m, "a.test") == "two" })
	if old.app.IsBootstrapped() {
		t.Fatal("the old app was still running when the new one served")
	}
}
