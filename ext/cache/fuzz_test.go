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
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	cachev1 "pocketone.dev/ext/contracts/cache/v1"
)

// checkIndex verifies every structure the lock guards. It holds at any moment the lock is free, IO in flight or not.
func checkIndex(s *store) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	listed := func(l *list.List, name string, el func(*entry) *list.Element) (map[*entry]bool, error) {
		seen := map[*entry]bool{}
		for x := l.Front(); x != nil; x = x.Next() {
			e := x.Value.(*entry)
			if seen[e] || el(e) != x {
				return nil, fmt.Errorf("%s: entry %q listed twice or its element doesn't match", name, e.key)
			}
			seen[e] = true
		}
		return seen, nil
	}
	inMem, err := listed(s.mem, "store mem", func(e *entry) *list.Element { return e.gel })
	if err != nil {
		return err
	}
	var memBytes, keyBytes, pending int64
	keys := 0
	values := map[*value]bool{}
	items := map[any]bool{}
	held := map[*byte]bool{}
	for id, sp := range s.spaces {
		if sp.id != id || len(sp.entries) != sp.order.Len() {
			return fmt.Errorf("space %s: %d entries, %d in order", id, len(sp.entries), sp.order.Len())
		}
		mem, err := listed(sp.mem, id+" mem", func(e *entry) *list.Element { return e.mel })
		if err != nil {
			return err
		}
		dsk, err := listed(sp.disk, id+" disk", func(e *entry) *list.Element { return e.del })
		if err != nil {
			return err
		}
		var spMem, spKeys, spDisk int64
		for x := sp.order.Front(); x != nil; x = x.Next() {
			e := x.Value.(*entry)
			switch {
			case e.sp != sp || e.oel != x || sp.entries[e.key] != e || e.dead:
				return fmt.Errorf("space %s key %q: broken links", id, e.key)
			case e.inMem != mem[e] || e.inMem != inMem[e]:
				return fmt.Errorf("space %s key %q: in memory %v, but listed %v/%v", id, e.key, e.inMem, mem[e], inMem[e])
			case (e.disk != nil) != dsk[e]:
				return fmt.Errorf("space %s key %q: has a file %v, listed %v", id, e.key, e.disk != nil, dsk[e])
			case e.v == nil && e.disk == nil:
				return fmt.Errorf("space %s key %q: neither a value nor a file", id, e.key)
			case e.inMem && e.v == nil:
				return fmt.Errorf("space %s key %q: in memory without a value", id, e.key)
			case e.spilling() && !e.writing:
				return fmt.Errorf("space %s key %q: held for a write that isn't queued", id, e.key)
			case e.disk != nil && !e.disk.written && !e.writing:
				return fmt.Errorf("space %s key %q: on an unwritten file without writing", id, e.key)
			}
			if e.disk != nil {
				if _, ok := e.disk.refs[e]; !ok || s.blobs[e.disk.hash] != e.disk {
					return fmt.Errorf("space %s key %q: its file doesn't know it, or isn't indexed", id, e.key)
				}
				spDisk += e.disk.size
			}
			if e.v != nil {
				if values[e.v] {
					return fmt.Errorf("space %s key %q: its value is shared with another key", id, e.key)
				}
				values[e.v] = true
				if it := e.v.item; it != nil && reflect.TypeOf(it).Comparable() {
					if items[it] {
						return fmt.Errorf("space %s key %q: its item is held by another key", id, e.key)
					}
					items[it] = true
				}
				if len(e.v.data) > 0 {
					if held[&e.v.data[0]] {
						return fmt.Errorf("space %s key %q: its bytes are shared with another key", id, e.key)
					}
					held[&e.v.data[0]] = true
				}
			}
			if e.inMem {
				spMem += e.size
				memBytes += e.size
			}
			if e.spilling() {
				pending += e.size
			}
			spMem += e.cost()
			spKeys += e.cost()
			keyBytes += e.cost()
			keys++
		}
		if spMem != sp.memBytes || spKeys != sp.keyBytes || spDisk != sp.diskBytes {
			return fmt.Errorf("space %s: mem %d (counted %d), disk %d (counted %d)", id, sp.memBytes, spMem, sp.diskBytes, spDisk)
		}
		l := sp.lim
		if l.maxItems > 0 && len(sp.entries) > l.maxItems || over(l.maxMem, sp.memBytes) || over(l.maxDisk, sp.diskBytes) {
			return fmt.Errorf("space %s over its budget: %d entries, mem %d, disk %d (%+v)", id, len(sp.entries), sp.memBytes, sp.diskBytes, l)
		}
	}
	if len(inMem) != s.mem.Len() || memBytes != s.memBytes || keyBytes != s.keyBytes || keys != s.keys || pending != s.pending {
		return fmt.Errorf("totals: mem %d (counted %d), keys %d/%d (counted %d/%d), pending %d (counted %d)",
			s.memBytes, memBytes, s.keys, s.keyBytes, keys, keyBytes, s.pending, pending)
	}
	var diskBytes int64
	n := 0
	for x := s.disk.Front(); x != nil; x = x.Next() {
		b := x.Value.(*blob)
		switch {
		case b.el != x || s.blobs[b.hash] != b:
			return fmt.Errorf("file %s: listed but not indexed", b.hash.hex()[:8])
		case len(b.refs) == 0:
			return fmt.Errorf("file %s has no keys", b.hash.hex()[:8])
		}
		for e := range b.refs {
			if e.disk != b || e.dead {
				return fmt.Errorf("file %s: a ref doesn't point at it", b.hash.hex()[:8])
			}
		}
		diskBytes += b.size
		n++
	}
	if n != len(s.blobs) || diskBytes != s.diskBytes {
		return fmt.Errorf("files: %d indexed, %d listed; %d bytes (counted %d)", len(s.blobs), n, s.diskBytes, diskBytes)
	}
	if over(s.lim.maxMem, s.memBytes+s.keyBytes) && s.mem.Len() > 0 {
		return fmt.Errorf("over memory with values still in memory: %d+%d > %d", s.memBytes, s.keyBytes, s.lim.maxMem)
	}
	if over(s.lim.maxDisk, s.diskBytes) {
		return fmt.Errorf("disk %d over %d", s.diskBytes, s.lim.maxDisk)
	}
	if s.lim.dir == "" && len(s.blobs) > 0 {
		return fmt.Errorf("files without a disk tier")
	}
	return nil
}

// checkFiles verifies the disk tier's folder once IO is flushed: exactly one file per indexed blob, with its bytes.
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
	for _, sp := range s.spaces {
		for _, e := range sp.entries {
			if e.writing {
				return fmt.Errorf("key %q still writing after flush", e.key)
			}
		}
	}
	if s.lim.dir == "" {
		return nil
	}
	want := map[string]*blob{}
	for _, b := range s.blobs {
		if !b.written {
			return fmt.Errorf("file %s unwritten after flush", b.hash.hex()[:8])
		}
		want[s.blobPath(b.hash)] = b
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
		if err != nil || sha256.Sum256(data) != b.hash {
			return fmt.Errorf("file %s doesn't hold its bytes", filepath.Base(p))
		}
		found++
		return nil
	})
	if err != nil {
		return err
	}
	if found != len(want) {
		return fmt.Errorf("%d files indexed, %d on disk", len(want), found)
	}
	return nil
}

// palette are the values the fuzzer writes: few, so files are shared often; sizes from empty to larger than a small
// store. A value is written as bytes or as a thing (an item) of the same bytes.
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

// FuzzStore runs random operations on a small store (tiny budgets, so everything evicts, spills and shares files)
// against a model of what every key last held. The store may forget anything, but may never hold or return a value
// the model doesn't have: not a deleted, replaced, expired or other key's value. Values go in as bytes and as items.
// Every structure is checked after each step, and the disk tier's files on every flush.
func FuzzStore(f *testing.F) {
	f.Add([]byte{0, 1, 200, 1, 0, 0, 1, 2, 3, 4, 5, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{3, 2, 100, 1, 50, 1, 2, 1, 0, 30, 2, 2, 3, 0, 0, 5, 1, 2, 3, 0, 5, 5, 5, 0, 1, 1, 1, 7, 0, 9, 1, 2, 0})
	f.Add(bytes.Repeat([]byte{0, 5, 1, 5, 2, 4, 3, 1, 1, 0}, 12))
	f.Add(bytes.Repeat([]byte{1, 3, 9, 0, 255, 7, 2, 2, 8, 4, 6, 1, 0, 11}, 20))
	f.Add(bytes.Repeat([]byte{2, 1, 4, 1, 3, 12, 0, 1, 2, 10, 3, 0, 13, 1}, 20))
	f.Fuzz(func(t *testing.T, in []byte) {
		r := &fuzzReader{b: in}
		evs := []cachev1.Eviction{cachev1.LRU, cachev1.MRU, cachev1.FIFO, cachev1.LIFO}
		lim := poolLimits{eviction: evs[r.next()%4], maxMem: int64(600 + 8*r.next())}
		if r.next()%3 != 0 {
			lim.dir, lim.maxDisk = t.TempDir(), int64(150+12*r.next())
		}
		now := time.Unix(1000, 0)
		var clock sync.Mutex
		newS := func() *store {
			s, err := newStore(lim, slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatal(err)
			}
			s.now = func() time.Time { clock.Lock(); defer clock.Unlock(); return now }
			return s
		}
		s := newS()
		defer func() { s.close() }()

		spaceLim := func() spaceLimits {
			l := spaceLimits{eviction: evs[r.next()%4], maxItems: r.next() % 4}
			if n := r.next() % 4; n > 0 {
				l.maxMem = int64(150 + 250*n)
			}
			if n := r.next() % 3; n > 0 {
				l.maxDisk = int64(300 * n)
			}
			l.ttl = time.Duration(r.next()%3) * time.Second
			return l
		}
		const nSpaces, nKeys = 3, 4
		lims := make([]spaceLimits, nSpaces)
		spaces := make([]*space, nSpaces)
		for i := range spaces {
			lims[i] = spaceLim()
			spaces[i] = s.open(fmt.Sprintf("i%d/b", i), lims[i])
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
					case held && e.v != nil && !holds(e.v, palette[m.v]):
						t.Fatalf("step %d: space %d key %d holds the wrong value in memory", step, i, k)
					case held && e.v == nil && e.disk.hash != sha256.Sum256(palette[m.v]):
						t.Fatalf("step %d: space %d key %d points at the wrong file", step, i, k)
					case held && e.exp != m.exp:
						t.Fatalf("step %d: space %d key %d: expiry %d, model %d", step, i, k, e.exp, m.exp)
					case !held && known:
						delete(model, [2]int{i, k}) // forgotten: allowed
					}
				}
			}
		}
		checkRead := func(step, i, k int, got []byte, ok bool) {
			t.Helper()
			m, known := model[[2]int{i, k}]
			if ok && (!known || !bytes.Equal(got, palette[m.v])) {
				t.Fatalf("step %d: read space %d key %d returned %d bytes the model doesn't have", step, i, k, len(got))
			}
			if ok && m.exp != 0 && now.UnixNano() >= m.exp {
				t.Fatalf("step %d: read returned an expired value", step)
			}
		}

		for step := 0; !r.done() && step < 400; step++ {
			i, k := r.next()%nSpaces, r.next()%nKeys
			sp, key := spaces[i], fmt.Sprint(k)
			switch op := r.next() % 12; op {
			case 0, 1, 2, 10: // set, as bytes or (10) as an item
				v, ttl := r.next()%len(palette), ttls[r.next()%len(ttls)]
				if op == 10 {
					s.setItem(sp, key, &thing{s: string(palette[v])}, ttl)
				} else {
					s.set(sp, key, palette[v], ttl)
				}
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
			case 3, 4: // get
				got, ok := s.get(sp, key)
				checkRead(step, i, k, got, ok)
				if ok && len(got) > 0 {
					got[0] ^= 0xff // the caller owns it: agree checks the change didn't reach the store
				}
			case 5, 11: // get as an item
				it, ok, err := s.getItem(sp, key, thingDecoder)
				if err != nil {
					t.Fatalf("step %d: %v", step, err)
				}
				var got []byte
				if ok {
					got = []byte(it.(*thing).s)
				}
				checkRead(step, i, k, got, ok)
			case 6:
				s.del(sp, key)
				delete(model, [2]int{i, k})
			case 7:
				switch r.next() % 5 {
				case 0:
					s.clear(sp)
					for kk := range nKeys {
						delete(model, [2]int{i, kk})
					}
				case 1:
					s.clearPrefix(fmt.Sprintf("i%d/", i)) // the app's buckets
					for kk := range nKeys {
						delete(model, [2]int{i, kk})
					}
				default:
					lims[i] = spaceLim()
					s.open(sp.id, lims[i]) // an app restarting with other caps
				}
			case 8:
				clock.Lock()
				now = now.Add(time.Duration(1+r.next()%3) * time.Second)
				clock.Unlock() // expired keys may stay held until read or swept; agree checks their expiry, reads that none is returned
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

// holds reports whether v is want: bytes, or a thing of those bytes.
func holds(v *value, want []byte) bool {
	if v.item != nil {
		t, ok := v.item.(*thing)
		return ok && t.s == string(want)
	}
	return bytes.Equal(v.data, want)
}

// TestStress runs readers and writers on one store with a small memory and a disk tier, so values keep moving between
// memory, the IO queue and disk while others read them. Each worker owns keys no one else writes and checks every read
// of them exactly; around them, shared spaces churn with shared files, items, deletes, clears and new caps.
func TestStress(t *testing.T) {
	ops := 4000
	if testing.Short() {
		ops = 800
	}
	for _, mode := range []string{"lru", "fifo", "mru"} {
		t.Run(mode, func(t *testing.T) {
			ev := cachev1.Eviction(mode)
			s := newTestStore(t, poolLimits{eviction: ev, maxMem: 24 << 10, dir: t.TempDir(), maxDisk: 96 << 10})
			shared := make([]*space, 4)
			for i := range shared {
				shared[i] = s.open(fmt.Sprintf("shared%d/b", i), spaceLimits{eviction: cachev1.LRU, maxItems: 12, maxMem: 6 << 10})
			}
			const workers = 12
			var wg sync.WaitGroup
			errs := make(chan error, workers)
			for w := range workers {
				wg.Go(func() {
					r := rand.New(rand.NewPCG(uint64(w), 7))
					own := s.open(fmt.Sprintf("w%d/own", w), spaceLimits{eviction: cachev1.LRU, maxItems: 6, maxMem: 4 << 10})
					model := map[string][]byte{}
					for n := range ops {
						k := fmt.Sprint(r.IntN(10))
						switch r.IntN(12) {
						case 0, 1: // own: set a value only this worker could have written
							v := ownValue(w, k, n, r.IntN(3000))
							s.set(own, k, v, 0)
							model[k] = v
						case 2: // ... as an item
							v := ownValue(w, k, n, r.IntN(3000))
							s.setItem(own, k, &thing{s: string(v)}, 0)
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
						case 5: // own: read as an item
							it, ok, err := s.getItem(own, k, thingDecoder)
							want, known := model[k]
							if err != nil || ok && (!known || it.(*thing).s != string(want)) {
								errs <- fmt.Errorf("worker %d key %s: item read wrong (known %v, err %v)", w, k, known, err)
								return
							}
						case 6:
							s.del(own, k)
							delete(model, k)
						case 7, 8: // shared churn: the same bytes from many spaces and workers
							s.set(shared[r.IntN(len(shared))], k, palette[r.IntN(len(palette))], 0)
						case 9:
							if got, ok := s.get(shared[r.IntN(len(shared))], k); ok && !inPalette(got) {
								errs <- fmt.Errorf("shared read returned a value never written there")
								return
							}
						case 10:
							s.setItem(shared[r.IntN(len(shared))], k, &thing{s: string(palette[r.IntN(len(palette))])}, 0)
						case 11:
							switch r.IntN(20) {
							case 0:
								s.clear(shared[r.IntN(len(shared))])
							case 1:
								s.open(shared[r.IntN(len(shared))].id, spaceLimits{eviction: cachev1.FIFO, maxItems: 1 + r.IntN(12), maxMem: 6 << 10})
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
// longer holds, even when another key keeps that file alive.
func TestDiskReadRacesAChange(t *testing.T) {
	for _, change := range []string{"replace", "delete", "clear", "free the file"} {
		t.Run(change, func(t *testing.T) {
			s := newTestStore(t, poolLimits{maxMem: 1 << 20, dir: t.TempDir(), maxDisk: 1 << 20})
			a, b := s.open("inst1/x", room(2, 20)), s.open("inst2/x", room(2, 20))
			put(s, a, "k", "old")
			put(s, b, "k", "old") // keeps the file alive whatever happens to a's key
			put(s, a, "o", "pushes old to disk")
			put(s, b, "o", "pushes old to disk")
			s.flush()
			if bl := s.blobs[sum(sha256Of("old"))]; bl == nil || !bl.written || len(bl.refs) != 2 {
				t.Fatal("setup: old should be one file on disk for both keys")
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
				case "free the file":
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

// TestNoStaleReads: writers (one at a time per key) write rising versions to the same key in two spaces, so a file
// is shared until both move on, while tiny budgets push values through the disk tier. A reader that starts after
// version v was written must see v or later, or miss; never an older one, and never one deleted before it started.
func TestNoStaleReads(t *testing.T) {
	ops := 3000
	if testing.Short() {
		ops = 600
	}
	s := newTestStore(t, poolLimits{maxMem: 8 << 10, dir: t.TempDir(), maxDisk: 64 << 10})
	spaces := []*space{s.open("a/x", room(2, 2400)), s.open("b/x", room(2, 2400))}
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
