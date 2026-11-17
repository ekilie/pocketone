package cache

// The store is one per Extension value, so one per process: a single app, or every instance of a multisite host. Apps
// see it through namespaces ("spaces"), one per app and bucket, whose keys never meet another space's.
//
// Memory and disk follow different rules:
//   - In memory every key holds its own value: its bytes, or a Go value (cachev1.Item). Nothing in memory is shared
//     between keys, buckets or apps, so the code that owns a bucket owns what it holds.
//   - On disk values are files named by the SHA-256 of their bytes (a cachev1.Marshaler's bytes, for an item), kept
//     once however many keys, buckets or apps point at them, and removed when the last one goes. A key points at its
//     file with the value's version beside it: (space, key, version) -> sha.
//
//	space "inst1/pages", key "t:ab12"  (in memory: its own bytes)
//	space "inst2/pages", key "t:ab12" ─┐
//	space "inst3/wasm",  key "app"    ─┴─> file 9f3c… (one copy, two refs)
//
// Each bucket has its own budget: entries, memory, disk (spaceLimits), with its own eviction strategy. The store has
// a ceiling for memory and one for disk over every bucket together, so the total stays bounded however many apps
// there are; reaching one evicts by the store's strategy across buckets.
//
// The disk tier is a cold tier for the hot one in memory: a value reaches disk only when memory evicts it (marshaled
// then, for an item), goes back to memory (and leaves disk) when read, and the tier starts empty with the process.
//
// Locking: one mutex guards the index, the lists and the totals, and is never held across disk IO, marshaling or
// decoding. Files are written and removed by one IO goroutine (disk.go), in the order the lock decided them. An entry
// being written keeps its value in memory until its file is there. A file is read once at a time however many
// readers want it, and a key decoded once at a time; both are checked against the index again before they're used.

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
)

// keyOverhead is what a key costs in memory beyond its name: map slot, list elements, struct.
const keyOverhead = 160

// blobsDir is the disk tier's own folder inside disk_path: the only one the store ever wipes.
const blobsDir = "p1cache"

type sum [sha256.Size]byte

func (h sum) hex() string { return hex.EncodeToString(h[:]) }

// poolLimits are the store's, from the operator: ceilings over every bucket together.
type poolLimits struct {
	eviction cachev1.Eviction
	maxMem   int64 // values and key index in memory
	dir      string
	maxDisk  int64
}

// maxPending is how many bytes evicted from memory may wait there for the disk tier; past it, they're dropped.
func (l poolLimits) maxPending() int64 { return max(l.maxMem/4, 4<<20) }

// spaceLimits are one app's bucket's budget.
type spaceLimits struct {
	eviction cachev1.Eviction
	maxItems int   // entries, in memory or on disk
	maxMem   int64 // its keys and the values it holds in memory
	maxDisk  int64 // the files its keys point at, each counted in full
	ttl      time.Duration
}

// value is a key's value in memory: an item, or bytes.
type value struct {
	data []byte
	item cachev1.Item
}

func (v *value) size() int64 {
	if v.item != nil {
		return max(v.item.CacheSize(), 0)
	}
	return int64(len(v.data))
}

// canMarshal reports whether v can go to disk: bytes, or an item that is a cachev1.Marshaler.
func (v *value) canMarshal() bool {
	if v.item == nil {
		return true
	}
	_, ok := v.item.(cachev1.Marshaler)
	return ok
}

// version is v's format: its item's CacheVersion, "" for bytes or an item that can't marshal.
func (v *value) version() string {
	if m, ok := v.item.(cachev1.Marshaler); ok {
		return m.CacheVersion()
	}
	return ""
}

func (v *value) bytes() ([]byte, error) {
	if v.item == nil {
		return v.data, nil
	}
	m, ok := v.item.(cachev1.Marshaler)
	if !ok {
		return nil, errors.New("item can't marshal")
	}
	return m.MarshalCache()
}

type entry struct {
	sp   *space
	key  string
	ver  string // its value's version, kept with its file
	exp  int64  // unix ns; 0 = never
	dead bool   // removed: work queued for it is dropped

	v     *value // in memory (inMem), or held while its file is written (spilling)
	size  int64  // v's size when it was counted
	inMem bool

	disk    *blob // its file, once attached
	writing bool  // a write of its value is queued or running

	decoding chan struct{} // a decode for this key in progress

	oel *list.Element // sp.order: every entry
	mel *list.Element // sp.mem and
	gel *list.Element // s.mem: entries in memory
	del *list.Element // sp.disk: entries with a file
}

func (e *entry) expired(now int64) bool { return e.exp != 0 && now >= e.exp }

func (e *entry) cost() int64 { return int64(len(e.key)) + keyOverhead }

// spilling reports whether e is out of memory's lists but holds its value until its file is written.
func (e *entry) spilling() bool { return !e.inMem && e.v != nil }

// space is one app's bucket.
type space struct {
	id    string // "<app id>/<bucket>"
	lim   spaceLimits
	order *list.List // every entry, newest (or most recently used) at the front
	mem   *list.List // entries in memory
	disk  *list.List // entries with a file

	entries   map[string]*entry
	memBytes  int64 // every key's cost, and the values in memory
	keyBytes  int64 // every key's cost
	diskBytes int64
}

// blob is one file of the disk tier.
type blob struct {
	hash    sum
	size    int64
	written bool
	refs    map[*entry]struct{}
	el      *list.Element // s.disk; nil once freed
	loading *fileRead     // a read of its file in progress
}

type fileRead struct {
	done chan struct{}
	data []byte
	err  error
}

type store struct {
	lim poolLimits
	log *slog.Logger
	now func() time.Time

	mu         sync.Mutex
	spaces     map[string]*space
	mem        *list.List // every entry in memory, by the store's strategy
	memBytes   int64      // values in memory
	keyBytes   int64      // the key index, always in memory
	keys       int
	pending    int64 // bytes of spilling entries
	blobs      map[sum]*blob
	disk       *list.List // blobs, by the store's strategy
	diskBytes  int64
	sinceSweep int

	io ioQueue

	beforeRead   func() // tests: called before a file read, off the lock
	beforeDecode func() // tests: called before a decode, off the lock
}

func newStore(lim poolLimits, log *slog.Logger) (*store, error) {
	s := &store{lim: lim, log: log, now: time.Now, spaces: map[string]*space{}, blobs: map[sum]*blob{},
		mem: list.New(), disk: list.New()}
	s.io.cond = sync.NewCond(&s.io.mu)
	if lim.dir == "" {
		return s, nil
	}
	s.lim.dir = filepath.Join(lim.dir, blobsDir)
	if err := wipe(s.lim.dir); err != nil {
		return nil, fmt.Errorf("cache: disk tier %s: %w", s.lim.dir, err)
	}
	return s, nil
}

func (s *store) space(id string) *space {
	sp, ok := s.spaces[id]
	if !ok {
		sp = &space{id: id, entries: map[string]*entry{}, order: list.New(), mem: list.New(), disk: list.New()}
		s.spaces[id] = sp
	}
	return sp
}

// open returns space id with lim, creating it. An app that starts again (or a store that replayed its index) reopens
// its spaces with their current limits; what they hold stays.
func (s *store) open(id string, lim spaceLimits) *space {
	s.mu.Lock()
	defer s.mu.Unlock()
	sp := s.space(id)
	sp.lim = lim
	s.fitSpace(sp, nil)
	return sp
}

// ---- reads ----

// get returns key's bytes; an item that can't marshal, or fails to, is a miss.
func (s *store) get(sp *space, key string) ([]byte, bool) {
	data, ok, _ := s.getBytes(sp, key)
	return data, ok
}

func (s *store) getBytes(sp *space, key string) ([]byte, bool, error) {
	v, ok, err := s.load(sp, key, nil)
	if err != nil || !ok || v.item == nil {
		return v.data, ok, err
	}
	if !v.canMarshal() {
		return nil, false, nil
	}
	data, err := v.bytes()
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// getItem returns key's item: the one in memory, or its bytes of dec's version decoded.
func (s *store) getItem(sp *space, key string, dec cachev1.Decoder) (cachev1.Item, bool, error) {
	v, ok, err := s.load(sp, key, &dec)
	return v.item, ok, err
}

// load reads key: bytes (the caller's copy) or, with dec, the key's item.
func (s *store) load(sp *space, key string, dec *cachev1.Decoder) (value, bool, error) {
	for range 8 { // a read that raced a change looks again; past a few, it's a miss
		v, ok, retry, err := s.tryGet(sp, key, dec)
		if !retry {
			return v, ok, err
		}
	}
	return value{}, false, nil
}

func (s *store) tryGet(sp *space, key string, dec *cachev1.Decoder) (value, bool, bool, error) {
	s.mu.Lock()
	e, found := sp.entries[key]
	if !found {
		s.mu.Unlock()
		return value{}, false, false, nil
	}
	if e.expired(s.now().UnixNano()) {
		s.removeEntry(e)
		s.mu.Unlock()
		return value{}, false, false, nil
	}
	s.touch(e)
	if e.v != nil && (dec == nil || e.v.item != nil) { // served from memory as it is
		v := *e.v
		if v.item == nil {
			v.data = bytes.Clone(v.data)
		}
		s.mu.Unlock()
		return v, true, false, nil
	}
	if dec != nil && e.ver != dec.Version { // bytes of another format
		s.mu.Unlock()
		return value{}, false, false, nil
	}
	if dec != nil && e.decoding != nil { // someone is decoding this key: wait, then look again
		ch := e.decoding
		s.mu.Unlock()
		<-ch
		return value{}, false, true, nil
	}
	var decoding chan struct{}
	if dec != nil {
		decoding = make(chan struct{})
		e.decoding = decoding
		defer func() {
			s.mu.Lock()
			if e.decoding == decoding {
				e.decoding = nil
			}
			close(decoding)
			s.mu.Unlock()
		}()
	}

	if e.v != nil { // bytes in memory, wanted as an item
		held := e.v
		data := bytes.Clone(held.data)
		dhook := s.beforeDecode
		s.mu.Unlock()
		if dhook != nil {
			dhook()
		}
		it, err := dec.Decode(data)
		if err != nil {
			return value{}, false, false, err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if sp.entries[key] == e && e.v == held { // still the key's value: its item from now on
			s.replaceValue(e, &value{item: it})
		}
		return value{item: it}, true, false, nil
	}

	// on disk: one read of the file for every reader of it, one decode for this key
	b := e.disk
	fr, reader := b.loading, false
	if fr == nil {
		fr, reader = &fileRead{done: make(chan struct{})}, true
		b.loading = fr
	}
	path, hook, dhook := s.blobPath(b.hash), s.beforeRead, s.beforeDecode
	s.mu.Unlock()
	if reader {
		if hook != nil {
			hook()
		}
		fr.data, fr.err = os.ReadFile(path)
		if fr.err == nil && sha256.Sum256(fr.data) != b.hash {
			fr.err = errors.New("file doesn't match its hash")
		}
		s.mu.Lock()
		b.loading = nil
		s.mu.Unlock()
		close(fr.done)
	} else {
		<-fr.done
	}
	var it cachev1.Item
	var derr error
	if fr.err == nil && dec != nil {
		if dhook != nil {
			dhook()
		}
		it, derr = dec.Decode(bytes.Clone(fr.data))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if fr.err != nil {
		if s.blobs[b.hash] == b {
			s.log.Warn("cache: unreadable disk entry dropped", "error", fr.err)
			s.dropBlob(b)
		}
		return value{}, false, false, nil
	}
	if sp.entries[key] != e || e.disk != b || e.v != nil { // changed while we read: the index decides
		return value{}, false, true, nil
	}
	if derr != nil { // the file is fine; this caller's decoder isn't
		return value{}, false, false, derr
	}
	v := &value{item: it}
	if it == nil {
		v.data = bytes.Clone(fr.data) // this key's own copy
	}
	s.promote(e, v)
	out := *v
	if out.item == nil {
		out.data = bytes.Clone(out.data)
	}
	return out, true, false, nil
}

// touch moves e to the front of its lists, for strategies where reads count.
func (s *store) touch(e *entry) {
	sp := e.sp
	if touches(sp.lim.eviction) {
		sp.order.MoveToFront(e.oel)
		if e.inMem {
			sp.mem.MoveToFront(e.mel)
		}
		if e.del != nil {
			sp.disk.MoveToFront(e.del)
		}
	}
	if touches(s.lim.eviction) {
		if e.inMem {
			s.mem.MoveToFront(e.gel)
		}
		if e.disk != nil && e.disk.el != nil {
			s.disk.MoveToFront(e.disk.el)
		}
	}
}

// promote brings e's value back to memory from its file, if it could fit, and lets go of the file.
func (s *store) promote(e *entry, v *value) {
	size := v.size()
	if !s.fits(e.sp, size) {
		return // could never fit
	}
	e.v, e.size = v, size
	s.addMem(e)
	s.detachDisk(e) // the disk tier is overflow only
	s.fitSpace(e.sp, e)
	s.fitMem(e)
}

// replaceValue swaps e's bytes in memory for v, their decoded item, keeping the totals right.
func (s *store) replaceValue(e *entry, v *value) {
	if !e.inMem {
		e.v = v // spilling: its file is being written from the bytes, which are equivalent
		return
	}
	s.removeMem(e)
	e.v, e.size = v, v.size()
	s.addMem(e)
	s.fitSpace(e.sp, nil) // the item may be larger than its bytes: it may go itself
	s.fitMem(nil)
}

// ---- writes ----

func (s *store) set(sp *space, key string, data []byte, ttl time.Duration) {
	s.put(sp, key, &value{data: bytes.Clone(data)}, ttl)
}

// setItem stores item under key as it is.
func (s *store) setItem(sp *space, key string, item cachev1.Item, ttl time.Duration) {
	s.put(sp, key, &value{item: item}, ttl)
}

func (s *store) put(sp *space, key string, v *value, ttl time.Duration) {
	ver, size := v.version(), v.size() // the item's own methods, off the lock
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := sp.entries[key]; ok {
		s.removeEntry(old)
	}
	if ttl < 0 {
		return
	}
	if ttl == 0 {
		ttl = sp.lim.ttl
	}
	now := s.now().UnixNano()
	if s.sinceSweep++; s.sinceSweep > s.keys+16 { // amortised: one sweep per (number of keys) writes
		s.sweep(now)
	}
	e := &entry{sp: sp, key: key, ver: ver, v: v, size: size}
	if ttl > 0 {
		e.exp = now + int64(ttl)
	}
	if over(sp.lim.maxMem, e.cost()) || over(s.lim.maxMem, e.cost()+s.keyBytes) {
		return // not even its key fits
	}
	s.attach(e)
	if !s.fits(sp, e.size) {
		s.spill(e) // never fits in memory: straight to disk, or not kept
	} else {
		s.addMem(e)
	}
	if e.dead {
		return
	}
	s.fitSpace(sp, e)
	s.fitMem(e)
}

func (s *store) del(sp *space, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := sp.entries[key]; ok {
		s.removeEntry(e)
	}
}

func (s *store) clear(sp *space) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearSpace(sp)
}

// clearPrefix empties every space whose id starts with prefix ("<app id>/" for one app's buckets, "" for the store)
// and returns how many keys went.
func (s *store) clearPrefix(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, sp := range s.spaces {
		if strings.HasPrefix(id, prefix) {
			n += s.clearSpace(sp)
		}
	}
	return n
}

// clearID empties space id, opened or not, and returns how many keys went.
func (s *store) clearID(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sp, ok := s.spaces[id]; ok {
		return s.clearSpace(sp)
	}
	return 0
}

func (s *store) clearSpace(sp *space) int {
	n := len(sp.entries)
	for _, e := range sp.entries {
		s.removeEntry(e)
	}
	return n
}

// ---- bookkeeping, under the lock ----

// touches reports whether reads reorder entries under ev (LRU, MRU) rather than only writes (FIFO, LIFO).
func touches(ev cachev1.Eviction) bool { return ev == "" || ev == cachev1.LRU || ev == cachev1.MRU }

// over reports whether n exceeds limit (0 = no limit).
func over(limit, n int64) bool { return limit > 0 && n > limit }

// victim is the element strategy ev evicts from l, skipping skip.
func victim(l *list.List, ev cachev1.Eviction, skip any) *list.Element {
	fromFront := ev == cachev1.MRU || ev == cachev1.LIFO
	el := l.Back()
	if fromFront {
		el = l.Front()
	}
	for el != nil && el.Value == skip {
		if fromFront {
			el = el.Next()
		} else {
			el = el.Prev()
		}
	}
	return el
}

// attach indexes a new entry e in its space.
func (s *store) attach(e *entry) {
	sp := e.sp
	sp.entries[e.key] = e
	e.oel = sp.order.PushFront(e)
	sp.memBytes += e.cost()
	sp.keyBytes += e.cost()
	s.keyBytes += e.cost()
	s.keys++
}

// fits reports whether a value of size could be in memory beside every key of sp and of the store.
func (s *store) fits(sp *space, size int64) bool {
	return !over(sp.lim.maxMem, sp.keyBytes+size) && !over(s.lim.maxMem, s.keyBytes+size)
}

func (s *store) addMem(e *entry) {
	e.inMem = true
	e.mel = e.sp.mem.PushFront(e)
	e.gel = s.mem.PushFront(e)
	e.sp.memBytes += e.size
	s.memBytes += e.size
}

func (s *store) removeMem(e *entry) {
	e.inMem = false
	e.sp.mem.Remove(e.mel)
	s.mem.Remove(e.gel)
	e.mel, e.gel = nil, nil
	e.sp.memBytes -= e.size
	s.memBytes -= e.size
}

// removeEntry forgets e: its key, its value in memory, its file reference.
func (s *store) removeEntry(e *entry) {
	if e.dead {
		return
	}
	sp := e.sp
	if e.inMem {
		s.removeMem(e)
	} else if e.spilling() {
		s.pending -= e.size
	}
	if e.disk != nil {
		s.detachDisk(e)
	}
	sp.order.Remove(e.oel)
	delete(sp.entries, e.key)
	sp.memBytes -= e.cost()
	sp.keyBytes -= e.cost()
	s.keyBytes -= e.cost()
	s.keys--
	e.dead, e.v, e.writing = true, nil, false
}

// spill moves e out of memory (where it is, or would be): into a file, holding its value until it's written, or away
// when there's no disk tier, it can't marshal, or too much already waits for the disk.
func (s *store) spill(e *entry) {
	if e.inMem {
		s.removeMem(e)
	}
	tooBig := e.v.item == nil && (over(s.lim.maxDisk, e.size) || over(e.sp.lim.maxDisk, e.size)) // an item's size on disk is known once marshaled
	if s.lim.dir == "" || !e.v.canMarshal() || tooBig || s.pending+e.size > s.lim.maxPending() {
		e.v = nil // not held for a write: nothing pending to give back
		s.removeEntry(e)
		return
	}
	s.pending += e.size
	e.writing = true
	s.io.write(s, e)
}

// fitSpace evicts sp's own entries (never keep) until it's within its budget.
func (s *store) fitSpace(sp *space, keep *entry) {
	for sp.lim.maxItems > 0 && len(sp.entries) > sp.lim.maxItems {
		el := victim(sp.order, sp.lim.eviction, keep)
		if el == nil {
			break
		}
		s.removeEntry(el.Value.(*entry))
	}
	for over(sp.lim.maxMem, sp.memBytes) {
		if el := victim(sp.mem, sp.lim.eviction, keep); el != nil {
			s.spill(el.Value.(*entry))
			continue
		}
		el := victim(sp.order, sp.lim.eviction, keep) // only keys left in memory: drop some
		if el == nil {
			break
		}
		s.removeEntry(el.Value.(*entry))
	}
	for over(sp.lim.maxDisk, sp.diskBytes) {
		el := victim(sp.disk, sp.lim.eviction, keep)
		if el == nil {
			break
		}
		s.removeEntry(el.Value.(*entry))
	}
}

// fitMem moves entries out of memory (never keep) until the store is within its memory ceiling.
func (s *store) fitMem(keep *entry) {
	for over(s.lim.maxMem, s.memBytes+s.keyBytes) {
		el := victim(s.mem, s.lim.eviction, keep)
		if el == nil {
			return
		}
		s.spill(el.Value.(*entry))
	}
}

// sweep drops every expired key.
func (s *store) sweep(now int64) {
	s.sinceSweep = 0
	for _, sp := range s.spaces {
		for _, e := range sp.entries {
			if e.expired(now) {
				s.removeEntry(e)
			}
		}
	}
}

// close waits for every queued disk write and removal.
func (s *store) close() { s.flush() }

// flush waits until every queued disk write and removal is done.
func (s *store) flush() { s.io.flush() }
