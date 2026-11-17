package cache

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
)

// thing is a test item that can marshal: it counts how often it's marshaled, and can refuse.
type thing struct {
	s        string
	size     int64
	ver      string
	marshals atomic.Int32
	fail     bool
}

func (t *thing) MarshalCache() ([]byte, error) {
	t.marshals.Add(1)
	if t.fail {
		return nil, errors.New("won't marshal")
	}
	return []byte(t.s), nil
}

func (t *thing) CacheSize() int64 {
	if t.size != 0 {
		return t.size
	}
	return int64(len(t.s))
}

func (t *thing) CacheVersion() string { return t.ver }

// live is a test item that can't marshal: memory only.
type live struct{ n int }

func (*live) CacheSize() int64 { return 10 }

var decodes atomic.Int32

var thingDecoder = cachev1.Decoder{Decode: func(b []byte) (cachev1.Item, error) {
	decodes.Add(1)
	return &thing{s: string(b)}, nil
}}

func versionDecoder(v string) cachev1.Decoder {
	return cachev1.Decoder{Version: v, Decode: func(b []byte) (cachev1.Item, error) { return &thing{s: string(b), ver: v}, nil }}
}

func getThing(t *testing.T, s *store, sp *space, k string) *thing {
	t.Helper()
	it, ok, err := s.getItem(sp, k, thingDecoder)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		return nil
	}
	return it.(*thing)
}

func TestItemsKeptAsValues(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20})
	sp := s.open("/x", lru(0))
	v := &thing{s: "hello"}
	s.setItem(sp, "k", v, 0)
	if got := getThing(t, s, sp, "k"); got != v {
		t.Fatalf("GetItem returned %p, want the value set (%p)", got, v)
	}
	if v.marshals.Load() != 0 {
		t.Fatal("an item in memory was marshaled")
	}
	if got := val(s, sp, "k"); got != "hello" { // plain Get marshals
		t.Fatalf("Get of an item: %q", got)
	}

	// GetItem of bytes decodes them, once: the item is the key's from then on
	put(s, sp, "b", "bytes")
	before := decodes.Load()
	first := getThing(t, s, sp, "b")
	if first == nil || first.s != "bytes" || getThing(t, s, sp, "b") != first || decodes.Load() != before+1 {
		t.Fatal("GetItem of bytes should decode once and keep the item")
	}

	other := s.open("/y", lru(0))
	s.setItem(other, "k", v, 0) // the same item set in two buckets: the caller's to share, never the store's
	if err := checkIndex(s); err == nil {
		t.Fatal("checkIndex should notice one value under two keys") // proves the check below means something
	}
	s.del(other, "k")
	if err := checkIndex(s); err != nil {
		t.Fatal(err)
	}
}

func TestItemSizeCounts(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20})
	sp := s.open("/x", room(2, 1000))
	s.setItem(sp, "big", &thing{s: "x", size: 2000}, 0) // larger than the bucket: not kept
	if has(s, sp, "big") {
		t.Fatal("an item over the bucket's memory was kept")
	}
	s.setItem(sp, "a", &thing{s: "a", size: 600}, 0)
	s.setItem(sp, "b", &thing{s: "b", size: 600}, 0) // a goes to make room
	if has(s, sp, "a") || !has(s, sp, "b") {
		t.Fatal("CacheSize isn't counted against the bucket's memory")
	}
	if err := checkIndex(s); err != nil {
		t.Fatal(err)
	}
}

// An item that can't marshal lives in memory only: evicted, it's gone; Get of it misses.
func TestItemMemoryOnly(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20, writeBehind: true})
	sp := s.open("/x", room(2, 15))
	v := &live{n: 1}
	s.setItem(sp, "a", v, 0)
	if it, ok, _ := s.getItem(sp, "a", thingDecoder); !ok || it != v {
		t.Fatal("GetItem of a memory-only item")
	}
	if has(s, sp, "a") {
		t.Fatal("Get of an item that can't marshal should miss")
	}
	s.setItem(sp, "b", &live{n: 2}, 0) // evicts a: nowhere to go
	if _, ok, _ := s.getItem(sp, "a", thingDecoder); ok {
		t.Fatal("an evicted memory-only item came back")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
	if countFiles(t, s.lim.dir) != 1 { // the index only
		t.Fatal("a memory-only item reached disk")
	}
}

func TestItemDiskTier(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20})
	sp := s.open("/x", room(2, 10))
	v := &thing{s: "on disk"}
	s.setItem(sp, "k", v, 0)
	s.setItem(sp, "o", &thing{s: "other"}, 0) // k spills: marshaled once, by the IO goroutine
	s.flush()
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
	if n := v.marshals.Load(); n != 1 {
		t.Fatalf("spilling marshaled %d times, want 1", n)
	}
	s.del(sp, "o")
	before := decodes.Load()
	got := getThing(t, s, sp, "k") // read back, decoded, back in memory as the key's item
	if got == nil || got.s != "on disk" || got == v {
		t.Fatalf("read back from disk: %+v", got)
	}
	if again := getThing(t, s, sp, "k"); again != got || decodes.Load() != before+1 {
		t.Fatal("an item read back from disk isn't kept as an item")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}

// A decoder of another version never sees the bytes.
func TestItemVersions(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20})
	sp := s.open("/x", room(4, 20))
	s.setItem(sp, "k", &thing{s: "v1 data", ver: "v1", size: 1 << 20}, 0) // too big for memory: on disk with version v1
	s.flush()
	if _, ok, _ := s.getItem(sp, "k", versionDecoder("v2")); ok {
		t.Fatal("a v2 decoder read v1 bytes")
	}
	it, ok, _ := s.getItem(sp, "k", versionDecoder("v1"))
	if !ok || it.(*thing).s != "v1 data" {
		t.Fatal("a v1 decoder should read v1 bytes")
	}
	put(s, sp, "b", "plain") // bytes Set as bytes are version ""
	if _, ok, _ := s.getItem(sp, "b", versionDecoder("v1")); ok {
		t.Fatal("a v1 decoder read unversioned bytes")
	}
	m := &live{}
	s.setItem(sp, "m", m, 0) // an item in memory is returned whatever the decoder's version
	if it, ok, _ := s.getItem(sp, "m", versionDecoder("v9")); !ok || it != m {
		t.Fatal("an item in memory should be returned as it is")
	}
}

func TestItemMarshalFailureDrops(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20})
	sp := s.open("/x", room(2, 1))
	s.setItem(sp, "bad", &thing{s: "x", fail: true}, 0)
	if _, _, err := s.getBytes(sp, "bad"); err == nil {
		t.Fatal("Get of an item that won't marshal: no error")
	}
	s.setItem(sp, "next", &thing{s: "y"}, 0) // bad spills, fails, goes
	s.flush()
	if _, ok, _ := s.getItem(sp, "bad", thingDecoder); ok {
		t.Fatal("an item that failed to spill is still there")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}

func TestItemDecodeError(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20})
	sp := s.open("/x", lru(0))
	put(s, sp, "k", "v")
	bad := cachev1.Decoder{Decode: func([]byte) (cachev1.Item, error) { return nil, errors.New("nope") }}
	if _, ok, err := s.getItem(sp, "k", bad); err == nil || ok {
		t.Fatal("a failing decoder: want an error")
	}
	if val(s, sp, "k") != "v" {
		t.Fatal("a failing decoder changed the value")
	}
}

// Many readers of one file read it once; many GetItems of one key decode it once and get the same item; keys that
// share a file each get their own item.
func TestReadsAndDecodesCoalesce(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20})
	a, b := s.open("a/x", room(2, 10)), s.open("b/x", room(2, 10))
	for _, sp := range []*space{a, b} {
		s.setItem(sp, "k", &thing{s: "shared file", size: 1 << 20}, 0) // too big for memory: both keys on one file
	}
	s.flush()
	if len(s.blobs) != 1 {
		t.Fatal("setup: want one file")
	}
	var reads, decoded atomic.Int32
	gate := make(chan struct{})
	s.beforeRead = func() { reads.Add(1); <-gate }
	s.beforeDecode = func() { decoded.Add(1) }
	const n = 10
	items := make([][]cachev1.Item, 2)
	var wg, started sync.WaitGroup
	for side, sp := range []*space{a, b} {
		items[side] = make([]cachev1.Item, n)
		for i := range n {
			started.Add(1)
			wg.Go(func() {
				started.Done()
				it, ok, err := s.getItem(sp, "k", thingDecoder)
				if err != nil || !ok {
					t.Errorf("reader %d/%d: %v %v", side, i, ok, err)
				}
				items[side][i] = it
			})
		}
	}
	started.Wait()
	for range 3 { // let the rest pile up behind the first read
		s.mu.Lock()
		s.mu.Unlock()
	}
	close(gate)
	wg.Wait()
	if r := reads.Load(); r != 1 {
		t.Errorf("%d file reads, want 1", r)
	}
	if d := decoded.Load(); d > 2 {
		t.Errorf("%d decodes, want at most one per key", d)
	}
	for side := range items {
		for i := range n {
			if items[side][i] != items[side][0] {
				t.Fatalf("key %d: readers got different items", side)
			}
		}
	}
	if items[0][0] == items[1][0] {
		t.Fatal("two keys sharing a file got the same item")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}

// TestItemsConcurrent mixes items and bytes on a tiny store with a disk tier under the race detector, then checks
// every structure and file.
func TestItemsConcurrent(t *testing.T) {
	for _, wt := range []bool{false, true} {
		t.Run(fmt.Sprintf("write-behind=%v", wt), func(t *testing.T) {
			s := newTestStore(t, poolLimits{maxMem: 4 << 10, dir: t.TempDir(), maxDisk: 16 << 10, writeBehind: wt})
			sp := s.open("/x", room(4, 1200))
			var wg sync.WaitGroup
			for g := range 8 {
				wg.Go(func() {
					r := rand.New(rand.NewPCG(uint64(g), 1))
					n := 2000
					if wt {
						n = 500 // every replacing Set waits for an fsync
					}
					for range n {
						k := fmt.Sprint(r.IntN(12))
						want := "v" + k
						switch r.IntN(7) {
						case 0:
							s.setItem(sp, k, &thing{s: want, size: int64(r.IntN(900))}, 0)
						case 1:
							s.set(sp, k, []byte(want), 0)
						case 2:
							s.del(sp, k)
						case 3:
							if v, ok := s.get(sp, k); ok && string(v) != want {
								t.Errorf("Get %s: %q", k, v)
							}
						case 4:
							s.setItem(sp, k, &live{}, 0)
						default:
							it, ok, err := s.getItem(sp, k, thingDecoder)
							if err != nil {
								t.Error(err)
							} else if th, isThing := it.(*thing); ok && isThing && th.s != want {
								t.Errorf("GetItem %s: %q", k, th.s)
							}
						}
					}
				})
			}
			wg.Wait()
			if err := checkFiles(s); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Writes are slotted: a key Set many times before the IO goroutine reaches it is marshaled once, with its latest value.
func TestWritesSlotted(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20, writeBehind: true})
	sp := s.open("/x", lru(0))
	release := make(chan struct{})
	s.io.beforeOp = func() { <-release }
	things := make([]*thing, 100)
	for i := range things {
		things[i] = &thing{s: fmt.Sprint("v", i)}
		s.setItem(sp, "k", things[i], 0)
	}
	s.io.mu.Lock()
	queued := len(s.io.ops)
	s.io.mu.Unlock()
	close(release)
	s.flush()
	if queued != 1 {
		t.Fatalf("%d writes queued for one key, want 1", queued)
	}
	for i, th := range things[:99] {
		if th.marshals.Load() != 0 {
			t.Fatalf("value %d was marshaled though a later Set replaced it", i)
		}
	}
	if things[99].marshals.Load() != 1 || sp.entries["k"].disk == nil {
		t.Fatal("the latest value should be marshaled and written once")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}
