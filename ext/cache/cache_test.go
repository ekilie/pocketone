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

	clearv1 "github.com/pockethost/pocketone/ext/contracts/cache/clear/v1"
	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

var ctx = context.Background()

func newTestStore(t testing.TB, lim poolLimits) *store {
	t.Helper()
	if lim.eviction == "" {
		lim.eviction = cachev1.LRU
	}
	s, err := newStore(lim, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.close) // before the disk tier's TempDir is removed (cleanups run last first)
	return s
}

func lru(n int) spaceLimits { return spaceLimits{eviction: cachev1.LRU, maxItems: n} }

// room is a bucket whose memory holds n one-letter keys and values bytes of values.
func room(n int, values int64) spaceLimits {
	return spaceLimits{eviction: cachev1.LRU, maxMem: int64(n)*(keyOverhead+1) + values}
}

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
		t.Fatal("clearing one app's bucket touched another's")
	}
}

// In memory every key has its own value, even when the bytes are the same.
func TestMemoryNeverShared(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20})
	wasm := string(make([]byte, 1000))
	spaces := []*space{s.open("inst1/wasm", lru(0)), s.open("inst2/wasm", lru(0)), s.open("inst3/other", lru(0))}
	for i, sp := range spaces {
		put(s, sp, fmt.Sprintf("key%d", i), wasm)
	}
	if s.memBytes != 3000 {
		t.Fatalf("%d bytes in memory, want 3000: one copy per key", s.memBytes)
	}
	if err := checkIndex(s); err != nil { // fails on any value or bytes shared between keys
		t.Fatal(err)
	}
}

// On disk the same bytes are one file, whichever keys, buckets or apps point at it.
func TestSameBytesOneFile(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20})
	wasm := string(make([]byte, 1000))
	spaces := []*space{s.open("inst1/wasm", room(1, 0)), s.open("inst2/wasm", room(1, 0)), s.open("inst3/other", room(1, 0))}
	for _, sp := range spaces {
		put(s, sp, "k", wasm) // larger than the bucket's memory: straight to disk
	}
	s.flush()
	if len(s.blobs) != 1 || s.diskBytes != 1000 {
		t.Fatalf("%d files, %d bytes: identical bytes should be one file", len(s.blobs), s.diskBytes)
	}
	for _, sp := range spaces {
		if sp.diskBytes != 1000 {
			t.Fatalf("space %s counts %d disk bytes, want the file in full", sp.id, sp.diskBytes)
		}
	}
	s.del(spaces[0], "k")
	s.del(spaces[1], "k")
	if val(s, spaces[2], "k") != wasm {
		t.Fatal("the file went while a key still points at it")
	}
	s.del(spaces[2], "k")
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
	if len(s.blobs) != 0 || countFiles(t, s.lim.dir) != 0 {
		t.Fatal("the file stayed after its last key went")
	}
}

// A bucket's budget is its own: filling one bucket evicts only its own values, to disk within its own disk budget.
func TestBucketBudgets(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20})
	busy := s.open("a/busy", spaceLimits{eviction: cachev1.LRU, maxMem: 2*(keyOverhead+2) + 200, maxDisk: 250})
	quiet := s.open("a/quiet", lru(0))
	put(s, quiet, "q", "stays")
	for i := range 10 {
		s.set(busy, fmt.Sprintf("k%d", i), []byte(fmt.Sprintf("%0100d", i)), 0)
		s.flush()
	}
	if !has(s, quiet, "q") {
		t.Fatal("a busy bucket evicted another bucket's value")
	}
	if busy.diskBytes > 250 || busy.memBytes > busy.lim.maxMem {
		t.Fatalf("busy bucket over its budget: mem %d, disk %d", busy.memBytes, busy.diskBytes)
	}
	if !has(s, busy, "k9") || has(s, busy, "k0") {
		t.Fatal("the busy bucket should keep its newest and drop its oldest")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}

// The store's memory ceiling holds over every bucket together, evicting across buckets.
func TestStoreCeiling(t *testing.T) {
	per := int64(100 + 1 + keyOverhead)
	s := newTestStore(t, poolLimits{maxMem: 3 * per})
	a, b := s.open("a/x", lru(0)), s.open("b/x", lru(0))
	for i := range 5 {
		sp := a
		if i%2 == 1 {
			sp = b
		}
		s.set(sp, fmt.Sprint(i), []byte(fmt.Sprintf("%0100d", i)), 0)
	}
	if s.keys != 3 || s.memBytes+s.keyBytes > s.lim.maxMem {
		t.Fatalf("%d keys, %d bytes: want 3 within %d", s.keys, s.memBytes+s.keyBytes, s.lim.maxMem)
	}
	s.set(a, "big", make([]byte, 1000), 0) // larger than the store: not kept without a disk tier
	if has(s, a, "big") || s.keys != 3 {
		t.Fatal("a value larger than the store was kept, or evicted others")
	}
	capped := s.open("/capped", spaceLimits{eviction: cachev1.LRU, maxMem: keyOverhead + 50})
	s.set(capped, "k", make([]byte, 100), 0)
	if has(s, capped, "k") {
		t.Fatal("a value larger than its bucket's memory was kept without a disk tier")
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

// The disk tier is overflow: a value read back goes to memory and leaves disk.
func TestDiskTier(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20})
	sp := s.open("/x", room(2, 11))
	put(s, sp, "a", "first")
	put(s, sp, "b", "second!") // a spills
	if e := sp.entries["a"]; e.inMem {
		t.Fatal("a should be out of memory")
	}
	if val(s, sp, "a") != "first" { // served while it's written, or read back from disk
		t.Fatal("a from the disk tier")
	}
	s.flush()
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
	put(s, sp, "b", "again!!") // replace b: a stays where it is
	s.flush()
	if sp.entries["a"].inMem {
		s.del(sp, "b")
	}
	got := val(s, sp, "a")
	s.flush()
	if got != "first" {
		t.Fatal("reading back from disk")
	}
	if e := sp.entries["a"]; e == nil || !e.inMem || e.disk != nil {
		t.Fatal("a value read back should be in memory and off disk")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
	s.clear(sp)
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
	if n := countFiles(t, s.lim.dir); n != 0 {
		t.Fatalf("%d files left after clearing every key", n)
	}
}

func TestDiskCeiling(t *testing.T) {
	s := newTestStore(t, poolLimits{eviction: cachev1.FIFO, maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 250})
	sp := s.open("/x", room(6, 100))
	for i := range 5 {
		s.set(sp, fmt.Sprint(i), []byte(fmt.Sprintf("%0100d", i)), 0)
		s.flush()
	}
	if len(s.blobs) != 2 || s.diskBytes > 250 {
		t.Fatalf("disk holds %d files, %d bytes; want 2 within 250", len(s.blobs), s.diskBytes)
	}
	if has(s, sp, "0") || !has(s, sp, "2") {
		t.Fatal("FIFO on disk should drop the oldest")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}

// The disk tier starts empty: what an earlier process left is removed, and nothing else in the folder is touched.
func TestDiskWipedOnStart(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: dir, maxDisk: 1 << 20})
	sp := s.open("/x", room(2, 3))
	put(s, sp, "a", "one")
	put(s, sp, "b", "two") // a spills
	s.flush()
	os.WriteFile(filepath.Join(s.lim.dir, ".tmp-1"), []byte("half"), 0o644)
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("mine"), 0o644)
	os.MkdirAll(filepath.Join(s.lim.dir, "zz"), 0o755)
	os.WriteFile(filepath.Join(s.lim.dir, "zz", "notes"), []byte("mine too"), 0o644)
	if countFiles(t, s.lim.dir) != 3 {
		t.Fatal("setup: want one blob file plus two others in the tier")
	}
	s2 := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: dir, maxDisk: 1 << 20})
	if len(s2.blobs) != 0 {
		t.Fatal("a new store should start empty")
	}
	if countFiles(t, s2.lim.dir) != 1 || countFiles(t, dir) != 2 {
		t.Fatalf("wipe removed the wrong files: %d in the tier, %d in all", countFiles(t, s2.lim.dir), countFiles(t, dir))
	}
}

// Values evicted from memory while the disk can't keep up are dropped, not held without bound.
func TestPendingBounded(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 64 << 20, dir: t.TempDir(), maxDisk: 1 << 30})
	big := s.lim.maxPending()/3 + 1
	sp := s.open("/x", room(1, big))
	release := make(chan struct{})
	s.io.beforeOp = func() { <-release } // hold the IO goroutine back
	for i := range 6 {
		v := make([]byte, big)
		v[0] = byte(i)
		s.set(sp, fmt.Sprint(i), v, 0) // each evicts the one before: queued while there's room, dropped after
	}
	s.mu.Lock()
	pending := s.pending
	s.mu.Unlock()
	close(release)
	if pending > s.lim.maxPending() {
		t.Fatalf("pending %d over %d", pending, s.lim.maxPending())
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}

func TestInstancesShareOneStore(t *testing.T) {
	x := New(Config{DiskPath: t.TempDir(), Buckets: map[string]BucketConfig{"wasm": {MaxMemory: 1024}}})
	t.Cleanup(func() { x.Close(ctx) })
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
	wasm := make([]byte, 4096) // over the bucket's memory: to disk
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
	x.store.flush()
	if len(x.store.spaces) != 2 || x.store.spaces["inst0/wasm"] == nil || x.store.spaces["inst1/wasm"] == nil {
		t.Fatalf("spaces: %v", x.store.spaces)
	}
	if n := len(x.store.blobs); n != 1 { // the module once on disk, for both instances
		t.Fatalf("%d files, want 1", n)
	}
	for i, app := range apps {
		b, _ := cachev1.Open(app, "wasm", cachev1.Options{})
		if v, ok, _ := b.Get(ctx, "mine"); !ok || string(v) != fmt.Sprint(i) {
			t.Fatalf("inst%d read %q", i, v)
		}
	}

	// an app clears its own buckets; the extension clears every app's
	_ = cachev1.Clear(ctx, apps[0])
	b0, _ := cachev1.Open(apps[0], "wasm", cachev1.Options{})
	b1, _ := cachev1.Open(apps[1], "wasm", cachev1.Options{})
	if _, ok, _ := b0.Get(ctx, "mine"); ok {
		t.Fatal("cachev1.Clear left the app's bucket")
	}
	if _, ok, _ := b1.Get(ctx, "mine"); !ok {
		t.Fatal("one app's Clear reached another app's bucket")
	}
	_ = x.Clear(ctx)
	if _, ok, _ := b1.Get(ctx, "mine"); ok {
		t.Fatal("the extension's Clear left a bucket")
	}
	if err := checkFiles(x.store); err != nil {
		t.Fatal(err)
	}
}

func TestProviderCaps(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	x := New(Config{Buckets: map[string]BucketConfig{"ops.tuned": {MaxItems: 7, MaxDisk: 1 << 20}}})
	layers := extkit.Config{{"defaults": map[string]any{"ttl": "5m", "max_memory": "1MiB"}, "buckets": map[string]any{"ops.tuned": map[string]any{"eviction": "fifo"}}}}
	if err := x.Register(app, layers); err != nil {
		t.Fatal(err)
	}
	extkit.AppScope(app).Seal(nil)
	c := cachev1.Contract.Must(app)

	got, err := c.Bucket("ops.tuned", cachev1.Options{MaxItems: 1000, Eviction: cachev1.LRU, TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	want := spaceLimits{eviction: cachev1.FIFO, maxItems: 7, maxMem: 1 << 20, maxDisk: 1 << 20, ttl: time.Hour}
	if l := got.(*bucket).sp.lim; l != want {
		t.Fatalf("caps %+v, want %+v (config over code, code over config defaults)", l, want)
	}
	if got.(*bucket).sp.id != "/ops.tuned" {
		t.Fatalf("a single app's space is %q", got.(*bucket).sp.id)
	}
	plain, _ := c.Bucket("plain", cachev1.Options{})
	if l := plain.(*bucket).sp.lim; l.ttl != 5*time.Minute || l.maxDisk != DefaultBucketMaxDisk || l.maxMem != 1<<20 {
		t.Fatalf("plain bucket %+v: want the config's defaults over the built-in ones", l)
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
	defer x.Close(ctx)
	if want := filepath.Join(app.DataDir(), "cache", blobsDir); x.store.lim.dir != want {
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
	if err := cachev1.Clear(ctx, app); err != nil {
		t.Fatal(err)
	}
}

func TestConfigDecoding(t *testing.T) {
	var y Config
	src := "max_memory: 512MiB\ndisk_path: /var/cache/p1\nmax_disk: 10GB\ndefaults: {max_memory: 64MiB, max_disk: 1GiB, ttl: 10m}\nbuckets:\n  a.b: {eviction: mru}\n"
	if err := yaml.Unmarshal([]byte(src), &y); err != nil {
		t.Fatal(err)
	}
	if y.MaxMemory != 512<<20 || y.MaxDisk != 10e9 || y.Defaults.MaxMemory != 64<<20 || y.Defaults.MaxDisk != 1<<30 ||
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

// cache.clear/v1 empties one of the calling app's buckets, or all of them, and nothing of another app's.
func TestClearContract(t *testing.T) {
	x := New(Config{})
	t.Cleanup(func() { x.Close(ctx) })
	apps := make([]core.App, 2)
	for i := range apps {
		app, err := tests.NewTestApp()
		if err != nil {
			t.Fatal(err)
		}
		defer app.Cleanup()
		extkit.SetAppID(app, fmt.Sprintf("inst%d", i))
		if err := x.Register(app, nil); err != nil {
			t.Fatal(err)
		}
		extkit.AppScope(app).Seal(nil)
		apps[i] = app
		for _, name := range []string{"a", "b"} {
			bk, _ := cachev1.Open(app, name, cachev1.Options{})
			_ = bk.Set(ctx, "k1", []byte("v"), 0)
			_ = bk.Set(ctx, "k2", []byte("v"), 0)
		}
	}
	c, su := clearv1.Contract.Must(apps[0]), extkit.Auth{Kind: "superuser"}
	if _, err := c.Clear(ctx, extkit.Auth{Kind: "record", Collection: "users"}, clearv1.ClearInput{}); err == nil {
		t.Fatal("an app user cleared the cache")
	}
	if r, err := c.Clear(ctx, su, clearv1.ClearInput{Bucket: "a"}); err != nil || r.Keys != 2 {
		t.Fatalf("clear bucket a: %+v %v", r, err)
	}
	if _, err := c.Clear(ctx, su, clearv1.ClearInput{Bucket: "../x"}); err == nil {
		t.Fatal("a bad bucket name was accepted")
	}
	if r, _ := c.Clear(ctx, su, clearv1.ClearInput{}); r.Keys != 2 { // b's two keys are left
		t.Fatalf("clear all: %d keys, want 2", r.Keys)
	}
	other, _ := cachev1.Open(apps[1], "a", cachev1.Options{})
	if _, ok, _ := other.Get(ctx, "k1"); !ok {
		t.Fatal("one app's clear reached another app")
	}
}
