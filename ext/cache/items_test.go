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

// thing is a test item: it counts how often it's marshaled, and can refuse.
type thing struct {
	s        string
	size     int64
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

var decodes atomic.Int32

func decodeThing(b []byte) (cachev1.Item, error) {
	decodes.Add(1)
	return &thing{s: string(b)}, nil
}

func getThing(t *testing.T, s *store, sp *space, k string) *thing {
	t.Helper()
	it, ok, err := s.getItem(sp, k, decodeThing)
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

	put(s, sp, "b", "bytes") // GetItem of bytes decodes
	if got := getThing(t, s, sp, "b"); got == nil || got.s != "bytes" {
		t.Fatal("GetItem of a byte value")
	}

	s.setItem(sp, "k2", v, 0) // the same item under two keys: two blobs, never shared
	if len(s.blobs) != 3 {
		t.Fatalf("%d blobs, want 3", len(s.blobs))
	}
	if err := checkIndex(s); err != nil {
		t.Fatal(err)
	}
}

func TestItemSizeCounts(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20})
	sp := s.open("/x", spaceLimits{eviction: cachev1.LRU, maxMem: 1000})
	s.setItem(sp, "big", &thing{s: "x", size: 2000}, 0) // larger than the bucket: not kept
	if has(s, sp, "big") {
		t.Fatal("an item over the bucket's cap was kept")
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

func TestItemDiskTier(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, maxItems: 1, dir: t.TempDir(), maxDisk: 1 << 20})
	sp := s.open("/x", lru(0))
	v := &thing{s: "on disk"}
	s.setItem(sp, "k", v, 0)
	s.setItem(sp, "k2", &thing{s: "second"}, 0) // k spills: marshaled once, by the IO goroutine
	s.flush()
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
	if n := v.marshals.Load(); n != 1 {
		t.Fatalf("spilling marshaled %d times, want 1", n)
	}
	before := decodes.Load()
	got := getThing(t, s, sp, "k") // read back, decoded, back in memory as an item
	if got == nil || got.s != "on disk" || got == v {
		t.Fatalf("read back from disk: %+v", got)
	}
	if again := getThing(t, s, sp, "k"); again != got || decodes.Load() != before+1 {
		t.Fatal("an item read back from disk isn't kept as an item")
	}
	s.flush()
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}

	// read back with a plain Get: bytes in memory, decoded again by each GetItem
	s.setItem(sp, "k3", &thing{s: "third"}, 0)
	s.flush()
	if val(s, sp, "k2") != "second" {
		t.Fatal("plain Get of an item on disk")
	}
	if getThing(t, s, sp, "k2").s != "second" {
		t.Fatal("GetItem of an item read back as bytes")
	}
	s.clear(sp)
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
	if n := countFiles(t, s.lim.dir); n != 0 {
		t.Fatalf("%d files left after clearing every key", n)
	}
}

func TestItemMarshalFailureDrops(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, maxItems: 1, dir: t.TempDir(), maxDisk: 1 << 20})
	sp := s.open("/x", lru(0))
	s.setItem(sp, "bad", &thing{s: "x", fail: true}, 0)
	if _, _, err := s.getBytes(sp, "bad"); err == nil {
		t.Fatal("Get of an item that won't marshal: no error")
	}
	s.setItem(sp, "next", &thing{s: "y"}, 0) // bad spills, fails, goes
	s.flush()
	if has(s, sp, "bad") {
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
	bad := func([]byte) (cachev1.Item, error) { return nil, errors.New("nope") }
	if _, ok, err := s.getItem(sp, "k", bad); err == nil || ok {
		t.Fatal("a failing decoder: want an error")
	}
	if !has(s, sp, "k") {
		t.Fatal("a failing decoder dropped the value")
	}
}

// TestItemsConcurrent mixes items and bytes on a tiny store with a disk tier under the race detector, then checks
// every structure and file.
func TestItemsConcurrent(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 4 << 10, maxItems: 4, dir: t.TempDir(), maxDisk: 16 << 10})
	sp := s.open("/x", lru(0))
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(g), 1))
			for range 2000 {
				k := fmt.Sprint(r.IntN(12))
				want := "v" + k
				switch r.IntN(6) {
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
				default:
					it, ok, err := s.getItem(sp, k, decodeThing)
					if err != nil {
						t.Error(err)
					} else if ok && it.(*thing).s != want {
						t.Errorf("GetItem %s: %q", k, it.(*thing).s)
					}
				}
			}
		}()
	}
	wg.Wait()
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}
