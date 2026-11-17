package cache

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
)

// checkIndex verifies every structure the lock guards. It holds at any moment the lock is free, IO in flight or not.
func checkIndex(s *store) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inList := func(l *list.List, state ...blobState) (map[*blob]bool, int64, error) {
		seen, total := map[*blob]bool{}, int64(0)
		for el := l.Front(); el != nil; el = el.Next() {
			b := el.Value.(*blob)
			if seen[b] {
				return nil, 0, fmt.Errorf("blob %s twice in a list", b.hash.hex()[:8])
			}
			if b.el != el {
				return nil, 0, fmt.Errorf("blob %s: el doesn't match its list element", b.hash.hex()[:8])
			}
			ok := false
			for _, st := range state {
				ok = ok || b.state == st
			}
			if !ok {
				return nil, 0, fmt.Errorf("blob %s in the wrong list for state %d", b.hash.hex()[:8], b.state)
			}
			seen[b] = true
			total += b.size
		}
		return seen, total, nil
	}
	mem, memBytes, err := inList(s.mem.order, inMemory)
	if err != nil {
		return err
	}
	disk, diskBytes, err := inList(s.disk.order, spilling, onDisk)
	if err != nil {
		return err
	}
	if memBytes != s.mem.bytes || diskBytes != s.disk.bytes {
		return fmt.Errorf("totals: mem %d (counted %d), disk %d (counted %d)", s.mem.bytes, memBytes, s.disk.bytes, diskBytes)
	}
	if len(mem)+len(disk) != len(s.blobs) {
		return fmt.Errorf("%d blobs indexed, %d in lists", len(s.blobs), len(mem)+len(disk))
	}
	var spillingBytes int64
	for h, b := range s.blobs {
		switch {
		case b.hash != h:
			return fmt.Errorf("blob indexed under the wrong hash")
		case len(b.refs) == 0:
			return fmt.Errorf("blob %s has no keys", h.hex()[:8])
		case !mem[b] && !disk[b]:
			return fmt.Errorf("blob %s in no list", h.hex()[:8])
		case b.state != onDisk && !b.isItem && (int64(len(b.data)) != b.size || sha256.Sum256(b.data) != b.hash):
			return fmt.Errorf("blob %s: held bytes don't match", h.hex()[:8])
		case b.isItem && len(b.refs) != 1:
			return fmt.Errorf("item %s shared by %d keys", h.hex()[:8], len(b.refs))
		case b.state != onDisk && b.isItem && (b.item == nil) == (b.data == nil):
			return fmt.Errorf("item %s in memory must hold exactly one of item or bytes", h.hex()[:8])
		case b.state == onDisk && (b.data != nil || b.item != nil):
			return fmt.Errorf("blob %s on disk still holds its bytes", h.hex()[:8])
		}
		if b.state == spilling {
			spillingBytes += b.size
		}
		for e := range b.refs {
			if e.b != b || e.sp.entries[e.key] != e {
				return fmt.Errorf("blob %s: a ref isn't an indexed key pointing at it", h.hex()[:8])
			}
		}
	}
	if s.pending < spillingBytes { // pending also counts writes of blobs freed since
		return fmt.Errorf("pending %d < spilling %d", s.pending, spillingBytes)
	}
	keys, keyBytes := 0, int64(0)
	for id, sp := range s.spaces {
		if sp.id != id || len(sp.entries) != sp.order.Len() {
			return fmt.Errorf("space %s: %d entries, %d in order", id, len(sp.entries), sp.order.Len())
		}
		var bytes int64
		for el := sp.order.Front(); el != nil; el = el.Next() {
			e := el.Value.(*entry)
			if e.sp != sp || e.el != el || sp.entries[e.key] != e || e.b == nil || s.blobs[e.b.hash] != e.b {
				return fmt.Errorf("space %s key %q: broken links", id, e.key)
			}
			if _, ok := e.b.refs[e]; !ok {
				return fmt.Errorf("space %s key %q: not among its blob's refs", id, e.key)
			}
			bytes += e.cost() + e.b.size
			keyBytes += e.cost()
			keys++
		}
		if bytes != sp.bytes {
			return fmt.Errorf("space %s: bytes %d, counted %d", id, sp.bytes, bytes)
		}
		if over(sp.lim.maxMem, sp.bytes) || sp.lim.maxItems > 0 && len(sp.entries) > sp.lim.maxItems {
			return fmt.Errorf("space %s over its caps: %d entries, %d bytes (%+v)", id, len(sp.entries), sp.bytes, sp.lim)
		}
	}
	if keys != s.keys || keyBytes != s.keyBytes {
		return fmt.Errorf("keys %d (counted %d), key bytes %d (counted %d)", s.keys, keys, s.keyBytes, keyBytes)
	}
	if over(s.lim.maxMem, s.mem.bytes+s.keyBytes) && s.mem.order.Len() > 0 {
		return fmt.Errorf("over memory with blobs still in memory: %d+%d > %d", s.mem.bytes, s.keyBytes, s.lim.maxMem)
	}
	if s.lim.maxItems > 0 && s.mem.order.Len() > s.lim.maxItems {
		return fmt.Errorf("%d blobs in memory, limit %d", s.mem.order.Len(), s.lim.maxItems)
	}
	if over(s.lim.maxDisk, s.disk.bytes) {
		return fmt.Errorf("disk %d over %d", s.disk.bytes, s.lim.maxDisk)
	}
	if s.lim.dir == "" && s.disk.order.Len() > 0 {
		return fmt.Errorf("blobs on disk without a disk tier")
	}
	return nil
}

// checkFiles verifies the disk tier's folder once IO is flushed: exactly one file per blob on disk, with its bytes.
func checkFiles(s *store) error {
	s.flush()
	if err := checkIndex(s); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != 0 {
		return fmt.Errorf("pending %d after flush", s.pending)
	}
	if s.lim.dir == "" {
		return nil
	}
	want := map[string]*blob{}
	for _, b := range s.blobs {
		switch b.state {
		case spilling:
			return fmt.Errorf("blob %s still spilling after flush", b.hash.hex()[:8])
		case onDisk:
			want[s.blobPath(b.hash)] = b
		}
	}
	found := 0
	err := filepath.WalkDir(s.lim.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, ok := want[p]
		if !ok {
			return fmt.Errorf("stray file %s", strings.TrimPrefix(p, s.lim.dir))
		}
		data, err := os.ReadFile(p)
		want := b.hash
		if b.isItem {
			want = b.fileSum
		}
		if err != nil || sha256.Sum256(data) != want {
			return fmt.Errorf("file %s doesn't hold its blob", filepath.Base(p))
		}
		found++
		return nil
	})
	if err != nil {
		return err
	}
	if found != len(want) {
		return fmt.Errorf("%d blobs on disk, %d files", len(want), found)
	}
	return nil
}

// palette are the values the fuzzer writes: few, so they're shared often; sizes from empty to larger than a small store.
var palette = func() [][]byte {
	sizes := []int{0, 7, 40, 120, 300, 900}
	out := make([][]byte, len(sizes))
	for i, n := range sizes {
		out[i] = bytes.Repeat([]byte{byte('a' + i)}, n)
	}
	return out
}()

type modelVal struct {
	v   int // palette index
	exp int64
}

type fuzzReader struct {
	b []byte
	i int
}

func (r *fuzzReader) next() int {
	if r.i >= len(r.b) {
		return 0
	}
	r.i++
	return int(r.b[r.i-1])
}

func (r *fuzzReader) done() bool { return r.i >= len(r.b) }

// FuzzStore runs random operations on a small store (tiny limits, so everything evicts, spills and shares) against
// a model of what every key last held. The store may forget anything, but may never hold or return a value the model
// doesn't have: not a deleted, replaced, expired or other key's value. Every structure is checked after each step,
// and the disk tier's files on every flush.
func FuzzStore(f *testing.F) {
	f.Add([]byte{0, 1, 200, 1, 0, 0, 1, 2, 3, 4, 5, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{3, 2, 100, 1, 50, 1, 2, 1, 0, 30, 2, 2, 3, 0, 0, 5, 1, 2, 3, 0, 5, 5, 5, 0, 1, 1, 1, 7, 0, 9, 1, 2, 0})
	f.Add(bytes.Repeat([]byte{0, 5, 1, 5, 2, 4, 3, 1, 1, 0}, 12))
	f.Add(bytes.Repeat([]byte{1, 3, 9, 0, 255, 7, 2, 2, 8, 4, 6, 1, 0}, 20))
	f.Fuzz(func(t *testing.T, in []byte) {
		r := &fuzzReader{b: in}
		evs := []cachev1.Eviction{cachev1.LRU, cachev1.MRU, cachev1.FIFO, cachev1.LIFO}
		lim := poolLimits{
			eviction: evs[r.next()%4],
			maxItems: r.next() % 5,
			maxMem:   int64(200 + 8*r.next()),
		}
		if r.next()%3 != 0 {
			lim.dir, lim.maxDisk = t.TempDir(), int64(150+12*r.next())
		}
		s, err := newStore(lim, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		now := time.Unix(1000, 0)
		var clock sync.Mutex
		s.now = func() time.Time { clock.Lock(); defer clock.Unlock(); return now }
		defer s.flush()

		spaceLim := func() spaceLimits {
			l := spaceLimits{eviction: evs[r.next()%4], maxItems: r.next() % 4}
			if n := r.next() % 4; n > 0 {
				l.maxMem = int64(150 + 250*n)
			}
			l.ttl = time.Duration(r.next()%3) * time.Second
			return l
		}
		const nSpaces, nKeys = 3, 4
		spaces := make([]*space, nSpaces)
		for i := range spaces {
			spaces[i] = s.open(fmt.Sprintf("i%d/b", i), spaceLim())
		}
		model := map[[2]int]modelVal{}
		ttls := []time.Duration{0, -1, time.Second, 3 * time.Second}

		// agree drops from the model what the store forgot, and fails on anything the store holds that it shouldn't.
		agree := func(step int) {
			t.Helper()
			if err := checkIndex(s); err != nil {
				t.Fatalf("step %d: %v", step, err)
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			for i, sp := range spaces {
				for k := range nKeys {
					e, held := sp.entries[fmt.Sprint(k)]
					m, known := model[[2]int{i, k}]
					switch {
					case held && !known:
						t.Fatalf("step %d: space %d key %d holds a value the model deleted", step, i, k)
					case held && e.b.hash != sha256.Sum256(palette[m.v]):
						t.Fatalf("step %d: space %d key %d holds the wrong value", step, i, k)
					case held && e.exp != m.exp:
						t.Fatalf("step %d: space %d key %d: expiry %d, model %d", step, i, k, e.exp, m.exp)
					case !held && known:
						delete(model, [2]int{i, k}) // forgotten: allowed
					}
				}
			}
		}

		for step := 0; !r.done() && step < 400; step++ {
			i, k := r.next()%nSpaces, r.next()%nKeys
			sp, key := spaces[i], fmt.Sprint(k)
			switch op := r.next() % 10; op {
			case 0, 1, 2: // set
				v, ttl := r.next()%len(palette), ttls[r.next()%len(ttls)]
				s.set(sp, key, palette[v], ttl)
				switch {
				case ttl < 0:
					delete(model, [2]int{i, k})
				default:
					if ttl == 0 {
						ttl = sp.lim.ttl
					}
					m := modelVal{v: v}
					if ttl > 0 {
						m.exp = now.UnixNano() + int64(ttl)
					}
					model[[2]int{i, k}] = m
				}
			case 3, 4, 5: // get
				got, ok := s.get(sp, key)
				m, known := model[[2]int{i, k}]
				if ok && (!known || !bytes.Equal(got, palette[m.v])) {
					t.Fatalf("step %d: get space %d key %d returned %d bytes the model doesn't have", step, i, k, len(got))
				}
				if ok && m.exp != 0 && now.UnixNano() >= m.exp {
					t.Fatalf("step %d: get returned an expired value", step)
				}
				if ok && len(got) > 0 {
					got[0] ^= 0xff // the caller owns it: agree checks the change didn't reach the store
				}
			case 6:
				s.del(sp, key)
				delete(model, [2]int{i, k})
			case 7:
				if r.next()%4 == 0 {
					s.clear(sp)
					for kk := range nKeys {
						delete(model, [2]int{i, kk})
					}
				} else {
					s.open(sp.id, spaceLim()) // an app restarting with other caps
				}
			case 8:
				clock.Lock()
				now = now.Add(time.Duration(1+r.next()%3) * time.Second)
				clock.Unlock() // expired keys may stay held until read or swept; agree checks their expiry, get that none is returned
			case 9:
				if err := checkFiles(s); err != nil {
					t.Fatalf("step %d: %v", step, err)
				}
			}
			agree(step)
		}
		if err := checkFiles(s); err != nil {
			t.Fatalf("end: %v", err)
		}
	})
}

// TestStress runs readers and writers on one store with a small memory and a disk tier, so values keep moving between
// memory, the IO queue and disk while others read them. Each worker owns keys no one else writes and checks every read
// of them exactly; around them, shared spaces churn with shared values, deletes, clears and new caps.
func TestStress(t *testing.T) {
	ops := 4000
	if testing.Short() {
		ops = 800
	}
	for _, ev := range []cachev1.Eviction{cachev1.LRU, cachev1.FIFO, cachev1.MRU} {
		t.Run(string(ev), func(t *testing.T) {
			s := newTestStore(t, poolLimits{eviction: ev, maxMem: 24 << 10, maxItems: 20, dir: t.TempDir(), maxDisk: 96 << 10})
			shared := make([]*space, 4)
			for i := range shared {
				shared[i] = s.open(fmt.Sprintf("shared%d/b", i), spaceLimits{eviction: cachev1.LRU, maxItems: 12})
			}
			const workers = 12
			var wg sync.WaitGroup
			errs := make(chan error, workers)
			for w := range workers {
				wg.Go(func() {
					r := rand.New(rand.NewPCG(uint64(w), 7))
					own := s.open(fmt.Sprintf("w%d/own", w), spaceLimits{eviction: cachev1.LRU, maxItems: 6})
					model := map[string][]byte{}
					for n := range ops {
						k := fmt.Sprint(r.IntN(10))
						switch r.IntN(10) {
						case 0, 1, 2: // own: set a value only this worker could have written
							v := ownValue(w, k, n, r.IntN(3000))
							s.set(own, k, v, 0)
							model[k] = v
						case 3, 4: // own: read exactly
							got, ok := s.get(own, k)
							want, known := model[k]
							if ok && (!known || !bytes.Equal(got, want)) {
								errs <- fmt.Errorf("worker %d key %s: read a value it never wrote last (known %v)", w, k, known)
								return
							}
							if !ok {
								delete(model, k)
							}
						case 5:
							s.del(own, k)
							delete(model, k)
						case 6, 7: // shared churn: values shared across spaces and workers
							s.set(shared[r.IntN(len(shared))], k, palette[r.IntN(len(palette))], 0)
						case 8:
							if got, ok := s.get(shared[r.IntN(len(shared))], k); ok && !inPalette(got) {
								errs <- fmt.Errorf("shared read returned a value never written there")
								return
							}
						case 9:
							switch r.IntN(20) {
							case 0:
								s.clear(shared[r.IntN(len(shared))])
							case 1:
								s.open(shared[r.IntN(len(shared))].id, spaceLimits{eviction: cachev1.FIFO, maxItems: 1 + r.IntN(12)})
							default:
								s.del(shared[r.IntN(len(shared))], k)
							}
						}
					}
				})
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatal(err)
			}
			if err := checkFiles(s); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// ownValue is a value only worker w writes for key k: its header names them, so a read can tell whose it is.
func ownValue(w int, k string, n, size int) []byte {
	v := make([]byte, 16+len(k)+size)
	binary.BigEndian.PutUint32(v, uint32(w))
	binary.BigEndian.PutUint32(v[4:], uint32(n))
	copy(v[16:], k)
	for i := 16 + len(k); i < len(v); i++ {
		v[i] = byte(n + i)
	}
	return v
}

func inPalette(v []byte) bool {
	for _, p := range palette {
		if bytes.Equal(v, p) {
			return true
		}
	}
	return false
}

// A disk read happens off the lock. Whatever happens to the key meanwhile, the read never returns what the key no
// longer holds, even when another key keeps that value alive.
func TestDiskReadRacesAChange(t *testing.T) {
	for _, change := range []string{"replace", "delete", "clear", "free the value"} {
		t.Run(change, func(t *testing.T) {
			s := newTestStore(t, poolLimits{maxMem: 1 << 20, maxItems: 1, dir: t.TempDir(), maxDisk: 1 << 20})
			a, b := s.open("inst1/x", lru(0)), s.open("inst2/x", lru(0))
			put(s, a, "k", "old")
			put(s, b, "k", "old") // keeps "old" alive whatever happens to a's key
			put(s, a, "other", "pushes old to disk")
			s.flush()
			if bl := s.blobs[sum(sha256Of("old"))]; bl == nil || bl.state != onDisk {
				t.Fatal("setup: old should be on disk")
			}
			once := false
			s.beforeRead = func() {
				if once {
					return
				}
				once = true
				switch change {
				case "replace":
					put(s, a, "k", "new")
				case "delete":
					s.del(a, "k")
				case "clear":
					s.clear(a)
				case "free the value":
					s.del(a, "k")
					s.del(b, "k")
				}
			}
			got, ok := s.get(a, "k")
			if ok && string(got) != "new" {
				t.Fatalf("read returned %q after the key changed", got)
			}
			if change == "replace" && !ok {
				t.Fatal("the new value should be read")
			}
			if err := checkFiles(s); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestNoStaleReads: writers (one at a time per key) write rising versions to the same key in two spaces, so a value
// is shared until both move on, while a tiny memory pushes values through the disk tier. A reader that starts after
// version v was written must see v or later, or miss; never an older one, and never one deleted before it started.
func TestNoStaleReads(t *testing.T) {
	ops := 3000
	if testing.Short() {
		ops = 600
	}
	s := newTestStore(t, poolLimits{maxMem: 6 << 10, maxItems: 4, dir: t.TempDir(), maxDisk: 64 << 10})
	spaces := []*space{s.open("a/x", lru(0)), s.open("b/x", lru(0))}
	const nKeys = 6
	type keyState struct {
		mu    sync.Mutex // writers of this key
		floor uint64     // reads starting now must see at least this version (or miss); guarded by floorMu
	}
	var floorMu sync.Mutex
	keys := make([]*keyState, nKeys)
	for i := range keys {
		keys[i] = &keyState{}
	}
	version := func(v []byte) uint64 { return binary.BigEndian.Uint64(v) }
	value := func(ver uint64) []byte {
		v := make([]byte, 8+int(ver%5)*400)
		binary.BigEndian.PutUint64(v, ver)
		return v
	}
	var next sync.Mutex
	var counter uint64
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for w := range 4 { // writers
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(w), 1))
			for range ops {
				k := r.IntN(nKeys)
				ks := keys[k]
				ks.mu.Lock()
				next.Lock()
				counter++
				ver := counter
				next.Unlock()
				del := r.IntN(8) == 0
				for _, sp := range spaces {
					if del {
						s.del(sp, fmt.Sprint(k))
					} else {
						s.set(sp, fmt.Sprint(k), value(ver), 0)
					}
				}
				floorMu.Lock()
				ks.floor = ver // done: later reads may not see anything older
				floorMu.Unlock()
				ks.mu.Unlock()
			}
		})
	}
	for rd := range 8 { // readers
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(rd), 2))
			for range ops * 2 {
				k := r.IntN(nKeys)
				floorMu.Lock()
				floor := keys[k].floor
				floorMu.Unlock()
				got, ok := s.get(spaces[r.IntN(2)], fmt.Sprint(k))
				if ok && version(got) < floor {
					errs <- fmt.Errorf("key %d: read version %d after version %d was written", k, version(got), floor)
					return
				}
				if ok && !bytes.Equal(got, value(version(got))) {
					errs <- fmt.Errorf("key %d: corrupt value", k)
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}

// Many readers of one value on disk read its file once; the rest wait for that read.
func TestDiskReadsCollapse(t *testing.T) {
	s := newTestStore(t, poolLimits{maxMem: 1 << 20, maxItems: 1, dir: t.TempDir(), maxDisk: 1 << 20})
	sp := s.open("/x", lru(0))
	put(s, sp, "k", "on disk")
	put(s, sp, "other", "pushes it out")
	s.flush()
	var mu sync.Mutex
	reads := 0
	arrived := make(chan struct{})
	s.beforeRead = func() {
		mu.Lock()
		reads++
		mu.Unlock()
		<-arrived // hold the first read until every reader is in
	}
	var wg sync.WaitGroup
	var waiting sync.WaitGroup
	for range 20 {
		waiting.Add(1)
		wg.Go(func() {
			waiting.Done()
			if v, ok := s.get(sp, "k"); !ok || string(v) != "on disk" {
				t.Errorf("reader got %q %v", v, ok)
			}
		})
	}
	waiting.Wait()
	time.Sleep(50 * time.Millisecond) // let them reach the store
	close(arrived)
	wg.Wait()
	if reads != 1 {
		t.Fatalf("%d disk reads for one value, want 1", reads)
	}
}
