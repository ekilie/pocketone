package standard_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/contracts/instance/source/v1/sourcetest"
	"github.com/pockethost/pocketone/ext/pocker"
	"github.com/pockethost/pocketone/ext/sandbox"
	"github.com/pockethost/pocketone/ext/standard"
)

// pocker's isolate_config is every instance's base config: `mcp: {enabled: false}` turns mcp off on each instance,
// and an instance's own extensions.mcp turns it back on.
func TestPockerIsolateConfig(t *testing.T) {
	root := t.TempDir()
	a := pocker.Instance{ID: "a", Hosts: []string{"a.test"}}
	b := pocker.Instance{ID: "b", Hosts: []string{"b.test"}, Extensions: map[string]any{"mcp": map[string]any{"enabled": true}}}
	m := pocker.NewManager(sourcetest.ScopeOf(a, b), pocker.ManagerOptions{
		PBRoot:        root,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Standard:      standard.New(),
		IsolateConfig: map[string]map[string]any{"mcp": {"enabled": false}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)
	defer m.Shutdown()
	for deadline := time.Now().Add(10 * time.Second); !m.Ready(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("manager not ready")
		}
	}
	for _, host := range []string{"a.test", "b.test"} { // instances start on their first request
		m.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://"+host+"/api/health", nil))
	}
	logA, _ := os.ReadFile(filepath.Join(root, "a", "logs", "exec.log"))
	logB, _ := os.ReadFile(filepath.Join(root, "b", "logs", "exec.log"))
	if !strings.Contains(string(logA), "mcp off: enabled: false") {
		t.Errorf("a's log:\n%s", logA)
	}
	if strings.Contains(string(logB), "mcp off") || !strings.Contains(string(logB), " mcp,") {
		t.Errorf("b's log:\n%s", logB)
	}
}

// A section for an extension the instances don't run is refused.
func TestPockerIsolateConfigUnknownSection(t *testing.T) {
	t.Cleanup(sandbox.UninstallEgressPolicy) // the host installs it process-wide; it blocks loopback for later tests
	srv := &pocker.Server{Config: pocker.DefaultConfig(), Standard: standard.New()}
	srv.Config.PBRoot = t.TempDir()
	srv.Config.IsolateConfig = map[string]map[string]any{"app": {"dev": true}}
	srv.Source = sourcetest.New()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := srv.Run(ctx)
	if err == nil || !strings.Contains(err.Error(), `isolate_config: unknown section "app"`) {
		t.Fatalf("err = %v", err)
	}
}
