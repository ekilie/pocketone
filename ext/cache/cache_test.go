package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"gopkg.in/yaml.v3"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

var ctx = context.Background()

func mustBucket(t *testing.T, lim limits) *bucket {
	t.Helper()
	if lim.maxMem == 0 {
		lim.maxMem = DefaultMaxMemory
	}
	if lim.maxDisk == 0 {
		lim.maxDisk = DefaultMaxDisk
	}
	b, err := newBucket("test", lim, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func set(t *testing.T, b *bucket, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if err := b.Set(ctx, k, []byte("v"+k), 0); err != nil {
			t.Fatal(err)
		}
	}
}

func has(b *bucket, k string) bool {
	_, ok, _ := b.Get(ctx, k)
	return ok
}

// memKeys lists what memory holds, without touching use order.
func memKeys(b *bucket) map[string]bool {
	out := map[string]bool{}
	for k := range b.mem.items {
		out[k] = true
	}
	return out
}

func TestEviction(t *testing.T) {
	// Each: fill a, b, c (cap 3), read a, add d. Which one went?
	cases := []struct {
		ev   cachev1.Eviction
		gone string
	}{
		{cachev1.LRU, "b"},  // a was read, b is least recently used
		{cachev1.MRU, "a"},  // a was just read
		{cachev1.FIFO, "a"}, // a was written first; the read doesn't count
		{cachev1.LIFO, "c"}, // c was written last
	}
	for _, c := range cases {
		t.Run(string(c.ev), func(t *testing.T) {
			b := mustBucket(t, limits{eviction: c.ev, maxItems: 3})
			set(t, b, "a", "b", "c")
			if !has(b, "a") {
				t.Fatal("a missing before eviction")
			}
			set(t, b, "d")
			got := memKeys(b)
			if len(got) != 3 || got[c.gone] || !got["d"] {
				t.Fatalf("after d: %v, want %s evicted", got, c.gone)
			}
		})
	}
}

func TestMaxMemory(t *testing.T) {
	b := mustBucket(t, limits{eviction: cachev1.LRU, maxMem: 3 * (entryOverhead + 101)})
	for i := range 5 {
		_ = b.Set(ctx, fmt.Sprint(i), make([]byte, 100), 0)
	}
	if len(b.mem.items) != 3 || b.mem.bytes > b.lim.maxMem {
		t.Fatalf("items %d bytes %d, want 3 within %d", len(b.mem.items), b.mem.bytes, b.lim.maxMem)
	}
	_ = b.Set(ctx, "big", make([]byte, 1000), 0) // larger than the bucket: dropped without a disk tier
	if has(b, "big") || len(b.mem.items) != 3 {
		t.Fatal("an entry larger than the bucket was kept or evicted others")
	}
}

func TestTTL(t *testing.T) {
	now := time.Unix(1000, 0)
	b := mustBucket(t, limits{eviction: cachev1.LRU, ttl: time.Minute})
	b.now = func() time.Time { return now }
	_ = b.Set(ctx, "default", []byte("x"), 0)
	_ = b.Set(ctx, "short", []byte("x"), time.Second)
	_ = b.Set(ctx, "gone", []byte("x"), -1)
	now = now.Add(2 * time.Second)
	if has(b, "short") || !has(b, "default") || has(b, "gone") {
		t.Fatal("short should have expired, default not, gone never stored")
	}
	now = now.Add(time.Minute)
	if has(b, "default") {
		t.Fatal("default ttl not applied")
	}
}

func TestCopies(t *testing.T) {
	b := mustBucket(t, limits{eviction: cachev1.LRU})
	v := []byte("abc")
	_ = b.Set(ctx, "k", v, 0)
	v[0] = 'X'
	got, _, _ := b.Get(ctx, "k")
	got[1] = 'Y'
	again, _, _ := b.Get(ctx, "k")
	if string(again) != "abc" {
		t.Fatalf("got %q: the bucket shares memory with its callers", again)
	}
}

func TestDiskTier(t *testing.T) {
	dir := t.TempDir()
	lim := limits{eviction: cachev1.LRU, maxItems: 2, dir: dir}
	b := mustBucket(t, lim)
	set(t, b, "a", "b", "c") // a spills to disk
	if memKeys(b)["a"] || b.disk.items["a"] == nil {
		t.Fatal("a should be on disk")
	}
	v, ok, _ := b.Get(ctx, "a") // back to memory, b spills
	if !ok || string(v) != "va" {
		t.Fatalf("a from disk: %q %v", v, ok)
	}
	if !memKeys(b)["a"] || b.disk.items["a"] != nil || b.disk.items["b"] == nil {
		t.Fatal("a should move to memory and b to disk")
	}

	// a delete reaches the disk tier
	_ = b.Delete(ctx, "b")
	if has(b, "b") {
		t.Fatal("deleted b came back from disk")
	}

	// what's on disk survives a restart
	set(t, b, "d", "e") // a and c... spill
	onDisk := len(b.disk.items)
	b2 := mustBucket(t, lim)
	if len(b2.disk.items) != onDisk || onDisk == 0 {
		t.Fatalf("reloaded %d disk entries, want %d", len(b2.disk.items), onDisk)
	}
	for k := range b.disk.items {
		if v, ok, _ := b2.Get(ctx, k); !ok || string(v) != "v"+k {
			t.Fatalf("%s after restart: %q %v", k, v, ok)
		}
	}

	// clear empties both tiers
	_ = b2.Clear(ctx)
	entries, _ := os.ReadDir(dir)
	if len(b2.mem.items) != 0 || len(b2.disk.items) != 0 || len(entries) != 0 {
		t.Fatal("clear left entries")
	}
}

func TestDiskLimit(t *testing.T) {
	dir := t.TempDir()
	size := int64(diskHeader + 1 + 100)
	b := mustBucket(t, limits{eviction: cachev1.FIFO, maxItems: 1, dir: dir, maxDisk: 2 * size})
	for i := range 5 {
		_ = b.Set(ctx, fmt.Sprint(i), make([]byte, 100), 0)
	}
	if len(b.disk.items) != 2 || b.disk.bytes > 2*size {
		t.Fatalf("disk holds %d entries, %d bytes; want 2 within %d", len(b.disk.items), b.disk.bytes, 2*size)
	}
	if has(b, "0") || !has(b, "2") {
		t.Fatal("FIFO on disk should drop the oldest")
	}
}

func TestDiskExpiredDroppedOnLoad(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1000, 0)
	b := mustBucket(t, limits{eviction: cachev1.LRU, maxItems: 1, dir: dir})
	b.now = func() time.Time { return now }
	_ = b.Set(ctx, "a", []byte("x"), time.Second)
	_ = b.Set(ctx, "b", []byte("x"), 0) // a spills
	_ = os.WriteFile(filepath.Join(dir, "junk"), []byte("nope"), 0o644)
	b2, err := newBucket("test", limits{eviction: cachev1.LRU, maxMem: DefaultMaxMemory, maxDisk: DefaultMaxDisk, dir: dir}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	// b2 runs at real time, so a (expiring 1001s after 1970) has expired; the junk file isn't an entry
	if len(b2.disk.items) != 0 {
		t.Fatalf("disk after reload: %d entries, want 0", len(b2.disk.items))
	}
	if _, err := os.Stat(filepath.Join(dir, "junk")); !os.IsNotExist(err) {
		t.Fatal("junk file left in the disk tier")
	}
}

func TestProvider(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	x := New(Config{Buckets: map[string]BucketConfig{"ops.tuned": {MaxItems: 7, DiskPath: "cache"}}})
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
	lim := got.(*bucket).lim
	want := limits{eviction: cachev1.FIFO, maxItems: 7, maxMem: DefaultMaxMemory, ttl: time.Hour, maxDisk: DefaultMaxDisk,
		dir: filepath.Join(app.DataDir(), "cache", "ops.tuned")}
	if lim != want {
		t.Fatalf("limits %+v, want %+v (config over code, code over config defaults)", lim, want)
	}
	other, _ := c.Bucket("plain", cachev1.Options{})
	if l := other.(*bucket).lim; l.ttl != 5*time.Minute || l.dir != "" {
		t.Fatalf("plain bucket %+v: want config default ttl, no disk", l)
	}
	again, _ := c.Bucket("ops.tuned", cachev1.Options{})
	if again != got {
		t.Fatal("same name should give the same bucket")
	}
	if _, err := c.Bucket("../x", cachev1.Options{}); err == nil {
		t.Fatal("bad name accepted")
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
	src := "defaults: {max_memory: 64MiB, ttl: 10m}\nbuckets:\n  a.b: {max_disk: 1GB, eviction: mru}\n"
	if err := yaml.Unmarshal([]byte(src), &y); err != nil {
		t.Fatal(err)
	}
	if y.Defaults.MaxMemory != 64<<20 || time.Duration(y.Defaults.TTL) != 10*time.Minute || y.Buckets["a.b"].MaxDisk != 1e9 {
		t.Fatalf("yaml: %+v", y)
	}
	var j Config
	if err := json.Unmarshal([]byte(`{"defaults":{"max_memory":"1KiB","max_disk":2048,"ttl":"1s"}}`), &j); err != nil {
		t.Fatal(err)
	}
	if j.Defaults.MaxMemory != 1024 || j.Defaults.MaxDisk != 2048 || time.Duration(j.Defaults.TTL) != time.Second {
		t.Fatalf("json: %+v", j)
	}
	if _, err := ParseSize("lots"); err == nil {
		t.Fatal("bad size accepted")
	}
	bad := Config{Buckets: map[string]BucketConfig{"x": {Eviction: "random"}}}
	if bad.validate() == nil {
		t.Fatal("bad eviction accepted")
	}
}
