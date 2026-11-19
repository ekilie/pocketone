package standard_test

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/cache"
	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
	"github.com/pockethost/pocketone/ext/contracts/instance/source/v1/sourcetest"
	"github.com/pockethost/pocketone/ext/pocker"
	"github.com/pockethost/pocketone/ext/standard"
)

// template is a compiled value an instance caches: large in memory, small once marshaled.
type template struct{ src string }

func (t *template) CacheSize() int64              { return 64 << 10 }
func (t *template) MarshalCache() ([]byte, error) { return []byte(t.src), nil }
func (t *template) CacheVersion() string          { return "tpl/1" }

func decodeTemplate(b []byte) (*template, error) { return &template{src: string(b)}, nil }

// On a Pocker host the standard set's cache is hoisted to the host: one store for every instance, each instance's
// buckets kept apart under its id without its code knowing. Two instances caching the same compiled template in two
// buckets each hold their own objects, see only their own keys, and share one file on disk.
func TestPockerInstancesShareTheHostCache(t *testing.T) {
	root, cacheDir := t.TempDir(), t.TempDir()
	set := standard.New()
	set.Cache = cache.New(cache.Config{DiskPath: cacheDir}) // the host's store, with a disk tier
	t.Cleanup(func() { set.Cache.Close(context.Background()) })

	var mu sync.Mutex
	apps := map[string]core.App{}
	m := pocker.NewManager(sourcetest.ScopeOf(
		pocker.Instance{ID: "a", Hosts: []string{"a.test"}},
		pocker.Instance{ID: "b", Hosts: []string{"b.test"}},
	), pocker.ManagerOptions{
		PBRoot:   root,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Standard: set,
		Setup: func(inst pocker.Instance, app core.App) error {
			mu.Lock()
			apps[inst.ID] = app
			mu.Unlock()
			return nil
		},
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
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://"+host+"/api/health", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s /api/health: %d", host, rec.Code)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(apps) != 2 {
		t.Fatalf("%d instances started, want 2", len(apps))
	}

	got := map[string]*template{}
	for id, app := range apps {
		for _, name := range []string{"pages.compiled", "mail.templates"} {
			b, err := cachev1.OpenItems(app, name, cachev1.Options{MaxMemory: 4 << 10}, "tpl/1", decodeTemplate)
			if err != nil {
				t.Fatal(err)
			}
			if err := b.Set(ctx, "layout", &template{src: "<html>{{.}}</html>"}, 0); err != nil { // over the bucket's memory: to disk
				t.Fatal(err)
			}
			if err := b.Set(ctx, "whoami", &template{src: "instance " + id}, 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	set.Cache.Close(ctx) // waits for the disk tier's writes; the store stays open

	for id, app := range apps {
		for _, name := range []string{"pages.compiled", "mail.templates"} {
			b, _ := cachev1.OpenItems(app, name, cachev1.Options{}, "tpl/1", decodeTemplate)
			v, ok, err := b.Get(ctx, "layout")
			if err != nil || !ok || v.src != "<html>{{.}}</html>" {
				t.Fatalf("%s %s layout: %+v %v %v", id, name, v, ok, err)
			}
			if got[id+name] = v; v == nil {
				t.Fatal("nil template")
			}
			if w, ok, _ := b.Get(ctx, "whoami"); !ok || w.src != "instance "+id {
				t.Fatalf("%s %s read another instance's key: %+v", id, name, w)
			}
		}
	}
	seen := map[*template]string{}
	for k, v := range got {
		if other, dup := seen[v]; dup {
			t.Fatalf("%s and %s got the same object", k, other)
		}
		seen[v] = k
	}

	// on disk: the layout once for all four keys, and each instance's whoami (one file per distinct bytes)
	files := map[string]bool{}
	_ = filepath.WalkDir(filepath.Join(cacheDir, "cache"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := os.ReadFile(p)
			files[string(b)] = !files[string(b)] // flips back to false on a second file of the same bytes
		}
		return nil
	})
	want := []string{"<html>{{.}}</html>", "instance a", "instance b"}
	if len(files) != len(want) {
		t.Fatalf("disk tier holds %v, want one file each of %q", files, want)
	}
	for _, w := range want {
		if !files[w] {
			t.Fatalf("disk tier: %q missing or stored twice (%v)", w, files)
		}
	}
}
