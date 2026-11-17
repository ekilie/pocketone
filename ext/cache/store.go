package cache

// The store is one per Extension value, so one per process: a single app, or every instance of a multisite host. Apps
// see it through namespaces ("spaces"), one per app and bucket, whose keys never meet another space's. Underneath,
// values are blobs named by the SHA-256 of their bytes: identical bytes under any number of keys, in any number of
// spaces, are kept once and count once against the store's limits.
//
//	space "inst1/pages.compiled", key "t:ab12" ─┐
//	space "inst2/pages.compiled", key "t:ab12" ─┼─> blob sha256:9f3c… (one copy, refs = 3)
//	space "inst2/wasm", key "app.wasm"         ─┘
//
// Two kinds of eviction:
//   - The store's: when the store is over its memory or item limit, a blob leaves memory (by the store's strategy):
//     to the disk tier when there is one, else with every key that points at it.
//   - A space's: when one app's bucket is over its own caps, one of its keys goes (by the bucket's strategy). The blob
//     goes too only when nothing else points at it.
//
// The disk tier is overflow for memory: one file per blob, named by its hash. The key index is only in memory, so the
// tier starts empty with the process (its folder is wiped then).
//
// Locking: one mutex guards the index, the lists and the totals, and is never held across disk IO. Files are written
// and removed by one IO goroutine, in the order the lock decided them, so a write and a later removal of the same file
// never swap; it runs only while it has work. A blob being written ("spilling") keeps its bytes in memory until the
// file is there, and reads are served from them; reads of a blob on disk happen outside the lock, one per blob at a
// time, and are checked against the index again before they're returned.

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
)

// keyOverhead is what a key costs in memory beyond its name: map slot, list element, struct.
const keyOverhead = 96

// blobsDir is the disk tier's own folder inside disk_path: the only one the store ever wipes.
const blobsDir = "p1cache"

type sum [sha256.Size]byte

func (h sum) hex() string { return hex.EncodeToString(h[:]) }

// poolLimits are the store's, from the operator: the room every app shares.
type poolLimits struct {
	eviction cachev1.Eviction
	maxItems int   // blobs in memory; 0 = no cap
	maxMem   int64 // blob bytes and key index in memory
	dir      string
	maxDisk  int64
}

// maxPending is how many bytes may wait in memory for the disk tier; past it, what memory evicts is dropped instead.
func (l poolLimits) maxPending() int64 { return max(l.maxMem/4, 4<<20) }

// spaceLimits are one app's bucket's caps inside the store.
type spaceLimits struct {
	eviction cachev1.Eviction
	maxItems int
	maxMem   int64 // logical: every key's name and its value's full size, even when the value is shared
	ttl      time.Duration
}

type blobState uint8

const (
	inMemory blobState = iota // data held, in store.mem
	spilling                  // data held, in store.disk, its file being written
	onDisk                    // data nil, in store.disk, its file written
)

type blob struct {
	hash    sum
	size    int64
	data    []byte
	state   blobState
	refs    map[*entry]struct{}
	el      *list.Element // in store.mem or store.disk; nil once freed
	loading chan struct{} // a read of its file in progress
}

type entry struct {
	sp  *space
	key string
	b   *blob
	exp int64         // unix ns; 0 = never
	el  *list.Element // in sp.order
}

func (e *entry) expired(now int64) bool { return e.exp != 0 && now >= e.exp }

func (e *entry) cost() int64 { return int64(len(e.key)) + keyOverhead }

// space is one app's bucket: its keys in use order, newest at the front.
type space struct {
	id      string // "<app id>/<bucket>"
	lim     spaceLimits
	entries map[string]*entry
	order   *list.List
	bytes   int64
}

type blobList struct {
	order *list.List
	bytes int64
}

type store struct {
	lim poolLimits
	log *slog.Logger
	now func() time.Time

	mu         sync.Mutex
	blobs      map[sum]*blob
	mem        blobList
	disk       blobList // spilling and onDisk
	pending    int64    // bytes of spilling blobs, held in memory until written
	keyBytes   int64    // the key index, always in memory
	keys       int
	spaces     map[string]*space
	sinceSweep int

	io ioQueue

	beforeRead func() // tests: called before a disk read, off the lock
}

func newStore(lim poolLimits, log *slog.Logger) (*store, error) {
	s := &store{lim: lim, log: log, now: time.Now, blobs: map[sum]*blob{}, spaces: map[string]*space{},
		mem: blobList{order: list.New()}, disk: blobList{order: list.New()}}
	s.io.cond = sync.NewCond(&s.io.mu)
	if lim.dir != "" {
		s.lim.dir = filepath.Join(lim.dir, blobsDir)
		if err := wipe(s.lim.dir); err != nil {
			return nil, fmt.Errorf("cache: disk tier %s: %w", s.lim.dir, err)
		}
	}
	return s, nil
}

// open returns space id with lim, creating it. An app that starts again reopens its spaces with its current limits;
// what they hold stays.
func (s *store) open(id string, lim spaceLimits) *space {
	s.mu.Lock()
	defer s.mu.Unlock()
	sp, ok := s.spaces[id]
	if !ok {
		sp = &space{id: id, entries: map[string]*entry{}, order: list.New()}
		s.spaces[id] = sp
	}
	sp.lim = lim
	s.fitSpace(sp, nil)
	return sp
}

func (s *store) get(sp *space, key string) ([]byte, bool) {
	for range 4 { // a read that raced a change looks again; past a few, it's a miss
		data, ok, retry := s.tryGet(sp, key)
		if !retry {
			return data, ok
		}
	}
	return nil, false
}

func (s *store) tryGet(sp *space, key string) (data []byte, ok, retry bool) {
	s.mu.Lock()
	e, found := sp.entries[key]
	if !found {
		s.mu.Unlock()
		return nil, false, false
	}
	if e.expired(s.now().UnixNano()) {
		s.removeEntry(e)
		s.mu.Unlock()
		return nil, false, false
	}
	if touches(sp.lim.eviction) {
		sp.order.MoveToFront(e.el)
	}
	b := e.b
	switch {
	case b.state != onDisk: // in memory, or held while its file is written
		if b.state == inMemory && touches(s.lim.eviction) {
			s.mem.order.MoveToFront(b.el)
		}
		data = bytes.Clone(b.data)
		s.mu.Unlock()
		return data, true, false
	case b.loading != nil: // someone is reading it: wait, then look again
		ch := b.loading
		s.mu.Unlock()
		<-ch
		return nil, false, true
	}
	ch := make(chan struct{})
	b.loading = ch
	path, hook := s.blobPath(b.hash), s.beforeRead
	s.mu.Unlock()

	if hook != nil {
		hook()
	}
	data, err := os.ReadFile(path)
	if err == nil && sha256.Sum256(data) != b.hash {
		err = errors.New("blob doesn't match its hash")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	b.loading = nil
	close(ch)
	if s.blobs[b.hash] != b || b.state != onDisk { // freed or moved while we read: the index decides
		return nil, false, true
	}
	if err != nil {
		s.log.Warn("cache: unreadable disk entry dropped", "error", err)
		s.dropBlob(b)
		return nil, false, false
	}
	if sp.entries[key] != e { // replaced or deleted while we read
		return nil, false, true
	}
	if touches(s.lim.eviction) {
		s.disk.order.MoveToFront(b.el)
	}
	if !over(s.lim.maxMem, b.size+s.keyBytes) { // back to memory, unless it could never fit
		s.disk.order.Remove(b.el)
		s.disk.bytes -= b.size
		s.io.remove(s, path)
		b.data, b.state = data, inMemory
		b.el = s.mem.order.PushFront(b)
		s.mem.bytes += b.size
		s.fitMem(b)
	}
	return bytes.Clone(data), true, false
}

func (s *store) set(sp *space, key string, value []byte, ttl time.Duration) {
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
	e := &entry{sp: sp, key: key}
	if ttl > 0 {
		e.exp = now + int64(ttl)
	}
	if over(sp.lim.maxMem, e.cost()+int64(len(value))) {
		return // larger than the bucket may hold
	}

	h := sum(sha256.Sum256(value))
	b, shared := s.blobs[h]
	if !shared {
		b = &blob{hash: h, size: int64(len(value)), data: bytes.Clone(value), refs: map[*entry]struct{}{}}
		s.blobs[h] = b
	}
	s.attach(e, b)
	switch {
	case shared:
		if b.state == inMemory && touches(s.lim.eviction) {
			s.mem.order.MoveToFront(b.el)
		}
	case over(s.lim.maxMem, b.size+e.cost()): // never fits in memory: straight to disk, or not kept
		s.spill(b)
	default:
		b.el = s.mem.order.PushFront(b)
		s.mem.bytes += b.size
	}
	if e.b == nil { // dropped while making room
		return
	}
	s.fitSpace(sp, e)
	s.fitMem(nil)
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
	for _, e := range sp.entries {
		s.removeEntry(e)
	}
}

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

// attach links a new entry e to blob b and to its space.
func (s *store) attach(e *entry, b *blob) {
	e.b = b
	b.refs[e] = struct{}{}
	e.sp.entries[e.key] = e
	e.el = e.sp.order.PushFront(e)
	e.sp.bytes += e.cost() + b.size
	s.keyBytes += e.cost()
	s.keys++
}

// removeEntry unlinks e from its space and blob, freeing the blob when nothing else points at it.
func (s *store) removeEntry(e *entry) {
	b := e.b
	if b == nil {
		return
	}
	sp := e.sp
	sp.order.Remove(e.el)
	delete(sp.entries, e.key)
	sp.bytes -= e.cost() + b.size
	s.keyBytes -= e.cost()
	s.keys--
	delete(b.refs, e)
	e.b = nil
	if len(b.refs) == 0 {
		s.freeBlob(b)
	}
}

// freeBlob forgets b (no keys point at it any more), removing its file if it has or is getting one.
func (s *store) freeBlob(b *blob) {
	if s.blobs[b.hash] == b {
		delete(s.blobs, b.hash)
	}
	if b.el == nil {
		return
	}
	if b.state == inMemory {
		s.mem.order.Remove(b.el)
		s.mem.bytes -= b.size
	} else {
		s.disk.order.Remove(b.el)
		s.disk.bytes -= b.size
		s.io.remove(s, s.blobPath(b.hash))
	}
	b.el, b.data = nil, nil
}

// dropBlob evicts b from the store with every key that points at it.
func (s *store) dropBlob(b *blob) {
	for e := range b.refs {
		s.removeEntry(e) // the last one frees b
	}
	if len(b.refs) == 0 {
		s.freeBlob(b) // a blob that had no keys (never attached)
	}
}

// fitSpace evicts sp's own entries (never keep) until it's within its caps.
func (s *store) fitSpace(sp *space, keep *entry) {
	for over(sp.lim.maxMem, sp.bytes) || sp.lim.maxItems > 0 && len(sp.entries) > sp.lim.maxItems {
		el := victim(sp.order, sp.lim.eviction, keep)
		if el == nil {
			return
		}
		s.removeEntry(el.Value.(*entry))
	}
}

// fitMem moves blobs out of memory (never keep) until the store is within its memory and item limits.
func (s *store) fitMem(keep *blob) {
	for over(s.lim.maxMem, s.mem.bytes+s.keyBytes) || s.lim.maxItems > 0 && s.mem.order.Len() > s.lim.maxItems {
		el := victim(s.mem.order, s.lim.eviction, keep)
		if el == nil {
			return
		}
		b := el.Value.(*blob)
		s.mem.order.Remove(el)
		s.mem.bytes -= b.size
		b.el = nil
		s.spill(b)
	}
}

// spill moves b (in no list) to the disk tier: it holds its bytes while the IO goroutine writes its file. Without a
// disk tier, when it doesn't fit, or when too much is already waiting to be written, b goes with its keys.
func (s *store) spill(b *blob) {
	if s.lim.dir == "" || over(s.lim.maxDisk, b.size) || s.pending+b.size > s.lim.maxPending() {
		s.dropBlob(b)
		return
	}
	for over(s.lim.maxDisk, s.disk.bytes+b.size) {
		el := victim(s.disk.order, s.lim.eviction, nil)
		if el == nil {
			break
		}
		s.dropBlob(el.Value.(*blob))
	}
	if len(b.refs) == 0 { // dropping others can't reach b (in no list), but stay safe
		s.freeBlob(b)
		return
	}
	b.state = spilling
	b.el = s.disk.order.PushFront(b)
	s.disk.bytes += b.size
	s.pending += b.size
	s.io.write(s, b)
}

// written is the IO goroutine's report on b's file: b is on disk now, unless it changed meanwhile.
func (s *store) written(b *blob, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending -= b.size
	if s.blobs[b.hash] != b || b.state != spilling {
		return // freed meanwhile: its removal is queued after this write
	}
	if err != nil {
		s.log.Warn("cache: disk write failed, entry dropped", "error", err)
		s.dropBlob(b)
		return
	}
	b.state, b.data = onDisk, nil
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

// flush waits until every queued disk write and removal is done.
func (s *store) flush() { s.io.flush() }

func (s *store) blobPath(h sum) string {
	x := h.hex()
	return filepath.Join(s.lim.dir, x[:2], x)
}

// ---- disk IO, off the lock ----

// ioQueue runs file writes and removals in the order they were queued (under the store's lock), on one goroutine
// that runs only while there is work.
type ioQueue struct {
	mu       sync.Mutex
	cond     *sync.Cond
	ops      []ioOp
	running  bool
	beforeOp func() // tests: called before each op, off every lock
}

type ioOp struct {
	path string
	b    *blob  // write data to path, then report on b; nil = remove path
	data []byte // the bytes to write (taken when queued)
}

func (q *ioQueue) write(s *store, b *blob) {
	q.push(s, ioOp{path: s.blobPath(b.hash), b: b, data: b.data})
}

func (q *ioQueue) remove(s *store, path string) { q.push(s, ioOp{path: path}) }

func (q *ioQueue) push(s *store, op ioOp) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ops = append(q.ops, op)
	if !q.running {
		q.running = true
		go q.run(s)
	}
}

func (q *ioQueue) run(s *store) {
	for {
		q.mu.Lock()
		if len(q.ops) == 0 {
			q.running = false
			q.ops = nil
			q.cond.Broadcast()
			q.mu.Unlock()
			return
		}
		op := q.ops[0]
		q.ops[0] = ioOp{}
		q.ops = q.ops[1:]
		hook := q.beforeOp
		q.mu.Unlock()

		if hook != nil {
			hook()
		}
		if op.b == nil {
			_ = os.Remove(op.path)
			continue
		}
		s.written(op.b, writeAtomic(s.lim.dir, op.path, op.data))
	}
}

func (q *ioQueue) flush() {
	q.mu.Lock()
	for q.running {
		q.cond.Wait()
	}
	q.mu.Unlock()
}

func writeAtomic(dir, path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	if err := errors.Join(werr, f.Close()); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	return nil
}

// wipe empties the disk tier's folder: only what the store writes there (two-hex-digit folders of blob files, and
// temp files), so a disk_path pointed at the wrong place loses nothing else.
func wipe(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tops, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, d := range tops {
		p := filepath.Join(dir, d.Name())
		switch {
		case strings.HasPrefix(d.Name(), ".tmp-") && !d.IsDir():
			_ = os.Remove(p)
		case d.IsDir() && len(d.Name()) == 2 && isHex(d.Name()):
			files, err := os.ReadDir(p)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			for _, f := range files {
				if !f.IsDir() && len(f.Name()) == 2*sha256.Size && isHex(f.Name()) && f.Name()[:2] == d.Name() {
					_ = os.Remove(filepath.Join(p, f.Name()))
				}
			}
			_ = os.Remove(p) // only if now empty
		}
	}
	return nil
}

func isHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
