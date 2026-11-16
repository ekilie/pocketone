package pocker

import (
	"context"
	"github.com/pockethost/pocketone/ext/contracts/instance/source/v1/sourcetest"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type staticSource []Instance

func (s staticSource) Snapshot(context.Context) ([]Instance, error) { return s, nil }
func (s staticSource) Watch(context.Context) (<-chan Change, error) { return nil, nil }

func newTestManager(t *testing.T, dev bool, list ...Instance) *Manager {
	t.Helper()
	_, scope := sourcetest.Scope(list...)
	m := NewManager(scope, ManagerOptions{Standard: testStandard,
		PBRoot: t.TempDir(),
		Dev:    dev,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	runManager(t, m)
	return m
}

// runManager runs m (its instances from instance.source/v1 on its scope) until the test ends, and waits until the
// source's snapshot has loaded.
func runManager(t *testing.T, m *Manager) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go m.Run(ctx)
	waitUntil(t, "the instances loaded", m.Ready)
}

// waitUntil polls cond every 5ms for up to 5s.
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("gave up after 5s waiting for %s", what)
		}
	}
}

func req(host string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "http://"+host+"/api/health", nil)
}

func TestValidID(t *testing.T) {
	for _, ok := range []string{"a", "blog.localhost", "3f9a2c", "shop-1.example.com"} {
		if !ValidID(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "..", "a..b", "../etc", "a/b", "A", "-a", "a-", ".a", "a b", "a\\b"} {
		if ValidID(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestDevID(t *testing.T) {
	if DevID("BLOG.localhost:8090") != "blog.localhost" || DevID("blog.localhost.") != "blog.localhost" ||
		DevID("blog.localhost.:8090") != "blog.localhost" {
		t.Fatal("DevID must normalize case, port and trailing dot")
	}
	if DevID("bad_host") != "" {
		t.Fatal("unsafe hostnames have no dev id")
	}
}
