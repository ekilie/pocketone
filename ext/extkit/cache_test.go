package extkit

import (
	"os"
	"path/filepath"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

func TestCacheDirs(t *testing.T) {
	root := t.TempDir()
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: filepath.Join(root, "pb_data")})
	dir, err := CacheDir(app, "agent")
	if err != nil || dir != filepath.Join(root, "pb_cache", "agent") {
		t.Fatalf("CacheDir = %q, %v", dir, err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("not created:", err)
	}
	if _, err := CacheDir(app, "../x"); err == nil {
		t.Fatal("want an error for a path in the name")
	}
	// a single app is its own host
	if d, _ := HostCacheDir(app, "cache"); d != filepath.Join(root, "pb_cache", "cache") {
		t.Fatalf("HostCacheDir (no host) = %q", d)
	}
	// an instance of a host with a cache root uses the host's
	host := NewScope("multisite")
	host.SetCacheRoot(filepath.Join(root, "host", "pb_cache"))
	inst := core.NewBaseApp(core.BaseAppConfig{DataDir: filepath.Join(root, "i1", "pb_data")})
	AttachHost(inst, host, "i1")
	if d, _ := HostCacheDir(inst, "cache"); d != filepath.Join(root, "host", "pb_cache", "cache") {
		t.Fatalf("HostCacheDir (host) = %q", d)
	}
	// its own cache is hoisted out of its folder, into the host's pb_cache
	if d, _ := CacheDir(inst, "agent"); d != filepath.Join(root, "host", "pb_cache", "instances", "i1", "agent") {
		t.Fatalf("CacheDir (instance) = %q", d)
	}
	if r := InstanceCacheRoot(host, "i1"); r != filepath.Join(root, "host", "pb_cache", "instances", "i1") {
		t.Fatalf("InstanceCacheRoot = %q", r)
	}
	for _, bad := range []string{"", "..", "a/b"} {
		if r := InstanceCacheRoot(host, bad); r != "" {
			t.Fatalf("InstanceCacheRoot(%q) = %q", bad, r)
		}
	}
	if _, err := HostCacheDir(inst, InstancesCacheFolder); err == nil {
		t.Fatal("instances is reserved in the host's pb_cache")
	}
	// a host with no cache root leaves its instances' caches in their folders
	bare := NewScope("multisite")
	inst2 := core.NewBaseApp(core.BaseAppConfig{DataDir: filepath.Join(root, "i2", "pb_data")})
	AttachHost(inst2, bare, "i2")
	if d, _ := CacheDir(inst2, "agent"); d != filepath.Join(root, "i2", "pb_cache", "agent") {
		t.Fatalf("CacheDir (host without cache root) = %q", d)
	}
}

func TestCacheUsageAndClear(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"agent/threads/a.bin", "agent/b.bin", "cache/x"} {
		p := filepath.Join(root, f)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("1234"), 0o644)
	}
	u, err := CacheUsages(root)
	if err != nil || len(u) != 2 || u[0].Extension != "agent" || u[0].Files != 2 || u[0].Bytes != 8 {
		t.Fatalf("usages: %+v %v", u, err)
	}
	cleared, err := ClearCache(root, "agent")
	if err != nil || len(cleared) != 1 {
		t.Fatalf("clear: %v %v", cleared, err)
	}
	u, _ = CacheUsages(root)
	if u[0].Files != 0 || u[1].Files != 1 {
		t.Fatalf("after clear: %+v", u)
	}
	if _, err := os.Stat(filepath.Join(root, "agent")); err != nil {
		t.Fatal("clear removed the folder itself")
	}
	if u, err := CacheUsages(filepath.Join(root, "missing")); err != nil || u != nil {
		t.Fatalf("missing root: %v %v", u, err)
	}
}
