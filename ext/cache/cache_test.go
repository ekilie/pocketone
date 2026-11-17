package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"gopkg.in/yaml.v3"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

var ctx = context.Background()

func newTestStore(t *testing.T, lim poolLimits) *store {
	t.Helper()
	if lim.eviction == "" {
		lim.eviction = cachev1.LRU
	}
	s, err := newStore(lim, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func lru(n int) spaceLimits { return spaceLimits{eviction: cachev1.LRU, maxItems: n} }

func put(s *store, sp *space, k, v string) { s.set(sp, k, []byte(v), 0) }

func has(s *store, sp *space, k string) bool {
	_, ok := s.get(sp, k)
	return ok
}

func val(s *store, sp *space, k string) string {
	v, _ := s.get(sp, k)
	return string(v)
}

func TestSpaceEviction(t *testing.T) {
	// fill a, b, c (cap 3), read a, add d: which one went?
	for ev, gone := range map[cachev1.Eviction]string{cachev1.LRU: "b", cachev1.MRU: "a", cachev1.FIFO: "a", cachev1.LIFO: "c"} {
		t.Run(string(ev), func(t *testing.T) {
			s := newTestStore(t, poolLimits{maxMem: 1 << 20})
			sp := s.open("/x", spaceLimits{eviction: ev, maxItems: 3})
			for _, k := range []string{"a", "b", "c"} {
				put(s, sp, k, "v"+k)
			}
			if !has(s, sp, "a") {
				t.Fatal("a missing before eviction")
			}
			put(s, sp, "d", "vd")
			if len(sp.entries) != 3 || sp.entries[gone] != nil || sp.entries["d"] == nil {
				t.Fatalf("after d: %d entries, want %s evicted", len(sp.entries), gone)
			}
		})
	}
}

func TestSpacesApart(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20})
	a, b := s.open("inst1/x", lru(0)), s.open("inst2/x", lru(0))
	put(s, a, "k", "from a")
	put(s, b, "k", "from b")
	if val(s, a, "k") != "from a" || val(s, b, "k") != "from b" {
		t.Fatal("two apps' buckets of one name share keys")
	}
	s.clear(a)
	if has(s, a, "k") || !has(s, b, "k") {
		t.Fatal("clear reached another app")
	}
}

func TestSameBytesStoredOnce(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, maxItems: 1}) // room for one value
	wasm := string(make([]byte, 1000))
	spaces := []*space{s.open("inst1/wasm", lru(0)), s.open("inst2/wasm", lru(0)), s.open("inst3/other", lru(0))}
	for i, sp := range spaces {
		put(s, sp, fmt.Sprintf("key%d", i), wasm)
	}
	if len(s.blobs) != 1 || s.mem.bytes != 1000 {
		t.Fatalf("%d blobs, %d bytes: identical values should be kept once", len(s.blobs), s.mem.bytes)
	}
	for i, sp := range spaces {
		if val(s, sp, fmt.Sprintf("key%d", i)) != wasm {
			t.Fatalf("key%d lost: a shared value should take one slot", i)
		}
	}
	s.del(spaces[0], "key0")
	s.del(spaces[1], "key1")
	if len(s.blobs) != 1 || !has(s, spaces[2], "key2") {
		t.Fatal("the value went while a key still points at it")
	}
	s.del(spaces[2], "key2")
	if len(s.blobs) != 0 || s.mem.bytes != 0 || s.keyBytes != 0 {
		t.Fatalf("left behind: %d blobs, %d bytes, %d key bytes", len(s.blobs), s.mem.bytes, s.keyBytes)
	}
}

func TestStoreEvictionDropsEveryKey(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, maxItems: 1}) // no disk tier
	a, b := s.open("inst1/x", lru(0)), s.open("inst2/x", lru(0))
	put(s, a, "k", "shared")
	put(s, b, "k", "shared")
	put(s, a, "other", "new")
	if has(s, a, "k") || has(s, b, "k") || !has(s, a, "other") {
		t.Fatal("an evicted value should take every key pointing at it")
	}
}

func TestSpaceCapKeepsSharedValue(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20})
	a, b := s.open("inst1/x", lru(1)), s.open("inst2/x", lru(0))
	put(s, a, "k", "shared")
	put(s, b, "k", "shared")
	put(s, a, "k2", "mine") // a is over its own cap: its k goes, b's stays
	if has(s, a, "k") || !has(s, b, "k") || !has(s, a, "k2") {
		t.Fatal("a bucket's cap should evict only its own keys")
	}
}

func TestMemoryLimit(t *testing.T) {
	per := int64(100 + 1 + keyOverhead)
	s := newTestStore(t, poolLimits{maxMem: 3 * per})
	sp := s.open("/x", lru(0))
	for i := range 5 {
		s.set(sp, fmt.Sprint(i), []byte(fmt.Sprintf("%0100d", i)), 0)
	}
	if len(sp.entries) != 3 || s.mem.bytes+s.keyBytes > s.lim.maxMem {
		t.Fatalf("%d entries, %d bytes: want 3 within %d", len(sp.entries), s.mem.bytes+s.keyBytes, s.lim.maxMem)
	}
	s.set(sp, "big", make([]byte, 1000), 0) // larger than the store: not kept without a disk tier
	if has(s, sp, "big") || len(sp.entries) != 3 {
		t.Fatal("a value larger than the store was kept, or evicted others")
	}

	capped := s.open("/capped", spaceLimits{eviction: cachev1.LRU, maxMem: 50})
	s.set(capped, "k", make([]byte, 100), 0)
	if has(s, capped, "k") {
		t.Fatal("a value larger than its bucket's cap was kept")
	}
}

func TestTTL(t *testing.T) {
	now := time.Unix(1000, 0)
	s := newTestStore(t, poolLimits{maxMem: 1 << 20})
	s.now = func() time.Time { return now }
	sp := s.open("/x", spaceLimits{eviction: cachev1.LRU, ttl: time.Minute})
	s.set(sp, "default", []byte("x"), 0)
	s.set(sp, "short", []byte("y"), time.Second)
	s.set(sp, "gone", []byte("z"), -1)
	now = now.Add(2 * time.Second)
	if has(s, sp, "short") || !has(s, sp, "default") || has(s, sp, "gone") {
		t.Fatal("short should have expired, default not, gone never stored")
	}
	now = now.Add(time.Minute)
	if has(s, sp, "default") {
		t.Fatal("the bucket's ttl wasn't applied")
	}
}

func TestCopies(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20})
	sp := s.open("/x", lru(0))
	v := []byte("abc")
	s.set(sp, "k", v, 0)
	v[0] = 'X'
	got, _ := s.get(sp, "k")
	got[1] = 'Y'
	if val(s, sp, "k") != "abc" {
		t.Fatal("the store shares memory with its callers")
	}
}

func TestDiskTier(t *testing.T) {
	dir := t.TempDir()
	lim := poolLimits{maxMem: 1 << 20, maxItems: 1, dir: dir, maxDisk: 1 << 20}
	s := newTestStore(t, lim)
	a, b := s.open("inst1/x", lru(0)), s.open("inst2/x", lru(0))
	put(s, a, "k", "shared")
	put(s, b, "k", "shared")
	put(s, a, "k2", "second") // the shared value spills, with both its keys
	if bl := s.blobs[sum(sha256Of("shared"))]; bl == nil || !bl.disk {
		t.Fatal("the shared value should be on disk")
	}
	if val(s, b, "k") != "shared" { // back to memory; "second" spills
		t.Fatal("shared value from disk")
	}
	s.del(a, "k2")
	if has(s, a, "k2") {
		t.Fatal("a delete didn't reach the disk tier")
	}

	put(s, a, "k3", "third") // "shared" spills again
	s2 := newTestStore(t, lim)
	a2, b2 := s2.open("inst1/x", lru(0)), s2.open("inst2/x", lru(0))
	if val(s2, a2, "k") != "shared" || val(s2, b2, "k") != "shared" || len(s2.blobs) != 1 {
		t.Fatalf("after a restart: %q %q, %d blobs", val(s2, a2, "k"), val(s2, b2, "k"), len(s2.blobs))
	}
	if has(s2, a2, "k3") {
		t.Fatal("a value that was only in memory survived a restart")
	}
	s2.clear(a2)
	s2.clear(b2)
	if n := countFiles(t, dir); n != 0 {
		t.Fatalf("%d files left after clearing every key", n)
	}
}

func TestDiskLimit(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t, poolLimits{eviction: cachev1.FIFO, maxMem: 1 << 20, maxItems: 1, dir: dir, maxDisk: 250})
	sp := s.open("/x", lru(0))
	for i := range 5 {
		s.set(sp, fmt.Sprint(i), []byte(fmt.Sprintf("%0100d", i)), 0)
	}
	if s.disk.order.Len() != 2 || s.disk.bytes > 250 {
		t.Fatalf("disk holds %d values, %d bytes; want 2 within 250", s.disk.order.Len(), s.disk.bytes)
	}
	if has(s, sp, "0") || !has(s, sp, "2") {
		t.Fatal("FIFO on disk should drop the oldest")
	}
}

func TestLoadDropsJunkAndExpired(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1000, 0)
	lim := poolLimits{maxMem: 1 << 20, maxItems: 1, dir: dir, maxDisk: 1 << 20}
	s := newTestStore(t, lim)
	s.now = func() time.Time { return now }
	sp := s.open("/x", lru(0))
	s.set(sp, "a", []byte("x"), time.Second)
	s.set(sp, "b", []byte("y"), 0) // a spills
	os.MkdirAll(filepath.Join(dir, "keys", "zz"), 0o755)
	os.WriteFile(filepath.Join(dir, "keys", "zz", "junk"), []byte("nope"), 0o644)
	os.WriteFile(filepath.Join(dir, ".tmp-1"), []byte("half"), 0o644)
	s2 := newTestStore(t, lim) // real time: a's expiry (1001s after 1970) has passed
	if len(s2.blobs) != 0 || countFiles(t, dir) != 0 {
		t.Fatalf("%d blobs, %d files: expired records, orphan blobs and junk should go", len(s2.blobs), countFiles(t, dir))
	}
}

func TestInstancesShareOneStore(t *testing.T) {
	x := New(Config{})
	apps := make([]core.App, 2)
	for i := range apps {
		app, err := tests.NewTestApp()
		if err != nil {
			t.Fatal(err)
		}
		defer app.Cleanup()
		extkit.SetAppID(app, fmt.Sprintf("inst%d", i))
		// an instance can cap its buckets but not resize the shared store
		layer := extkit.Config{{"max_memory": "1KiB", "buckets": map[string]any{"wasm": map[string]any{"max_items": 5}}}}
		if err := x.Register(app, layer); err != nil {
			t.Fatal(err)
		}
		extkit.AppScope(app).Seal(nil)
		apps[i] = app
	}
	if x.store.lim.maxMem != DefaultMaxMemory {
		t.Fatalf("an instance's settings changed the shared store: %d", x.store.lim.maxMem)
	}
	wasm := make([]byte, 4096)
	for i, app := range apps {
		b, err := cachev1.Open(app, "wasm", cachev1.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if b.(*bucket).sp.lim.maxItems != 5 {
			t.Fatal("the instance's bucket cap wasn't applied")
		}
		_ = b.Set(ctx, "module", wasm, 0)
		_ = b.Set(ctx, "mine", []byte(fmt.Sprint(i)), 0)
	}
	if len(x.store.spaces) != 2 || x.store.spaces["inst0/wasm"] == nil || x.store.spaces["inst1/wasm"] == nil {
		t.Fatalf("spaces: %v", x.store.spaces)
	}
	if n := len(x.store.blobs); n != 3 { // the module once, and each instance's own value
		t.Fatalf("%d values stored, want 3", n)
	}
	for i, app := range apps {
		b, _ := cachev1.Open(app, "wasm", cachev1.Options{})
		if v, ok, _ := b.Get(ctx, "mine"); !ok || string(v) != fmt.Sprint(i) {
			t.Fatalf("inst%d read %q", i, v)
		}
	}
}

func TestProviderCaps(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	x := New(Config{Buckets: map[string]BucketConfig{"ops.tuned": {MaxItems: 7}}})
	layers := extkit.Config{{"defaults": map[string]any{"ttl": "5m"}, "buckets": map[string]any{"ops.tuned": map[string]any{"eviction": "fifo"}}}}
	if err := x.Register(app, layers); err != nil {
		t.Fatal(err)
	}
	extkit.AppScope(app).Seal(nil)
	c := cachev1.Contract.Must(app)

	got, err := c.Bucket("ops.tuned", cachev1.Options{MaxItems: 1000, Eviction: cachev1.LRU, TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	want := spaceLimits{eviction: cachev1.FIFO, maxItems: 7, ttl: time.Hour}
	if l := got.(*bucket).sp.lim; l != want {
		t.Fatalf("caps %+v, want %+v (config over code, code over config defaults)", l, want)
	}
	if got.(*bucket).sp.id != "/ops.tuned" {
		t.Fatalf("a single app's space is %q", got.(*bucket).sp.id)
	}
	plain, _ := c.Bucket("plain", cachev1.Options{})
	if l := plain.(*bucket).sp.lim; l.ttl != 5*time.Minute {
		t.Fatalf("plain bucket %+v: want the config's default ttl", l)
	}
	if again, _ := c.Bucket("ops.tuned", cachev1.Options{}); again != got {
		t.Fatal("same name should give the same bucket")
	}
	if _, err := c.Bucket("../x", cachev1.Options{}); err == nil {
		t.Fatal("bad name accepted")
	}
}

func TestSingleAppDiskUnderDataDir(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	x := New(Config{DiskPath: "cache"})
	if err := x.Register(app, nil); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(app.DataDir(), "cache"); x.store.lim.dir != want {
		t.Fatalf("disk tier at %s, want %s", x.store.lim.dir, want)
	}
}

func TestOpenWithoutProvider(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	extkit.AppScope(app).Seal(nil)
	b, err := cachev1.Open(app, "x", cachev1.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_ = b.Set(ctx, "k", []byte("v"), 0)
	if _, ok, _ := b.Get(ctx, "k"); ok {
		t.Fatal("no provider: every get misses")
	}
}

func TestConfigDecoding(t *testing.T) {
	var y Config
	src := "max_memory: 512MiB\ndisk_path: /var/cache/p1\nmax_disk: 10GB\ndefaults: {max_memory: 64MiB, ttl: 10m}\nbuckets:\n  a.b: {eviction: mru}\n"
	if err := yaml.Unmarshal([]byte(src), &y); err != nil {
		t.Fatal(err)
	}
	if y.MaxMemory != 512<<20 || y.MaxDisk != 10e9 || y.Defaults.MaxMemory != 64<<20 ||
		time.Duration(y.Defaults.TTL) != 10*time.Minute || y.Buckets["a.b"].Eviction != cachev1.MRU {
		t.Fatalf("yaml: %+v", y)
	}
	var j Config
	if err := json.Unmarshal([]byte(`{"defaults":{"max_memory":"1KiB","ttl":"1s"},"max_disk":2048}`), &j); err != nil {
		t.Fatal(err)
	}
	if j.Defaults.MaxMemory != 1024 || j.MaxDisk != 2048 || time.Duration(j.Defaults.TTL) != time.Second {
		t.Fatalf("json: %+v", j)
	}
	if _, err := ParseSize("lots"); err == nil {
		t.Fatal("bad size accepted")
	}
	for _, bad := range []Config{{Eviction: "random"}, {Buckets: map[string]BucketConfig{"x": {Eviction: "random"}}}} {
		if bad.validate() == nil {
			t.Fatalf("bad eviction accepted: %+v", bad)
		}
	}
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func sha256Of(s string) [32]byte { return sha256.Sum256([]byte(s)) }
