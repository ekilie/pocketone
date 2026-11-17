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
//   - The store's: when the store is over its memory or item limit, a blob goes (by the store's strategy), to the disk
//     tier when there is one, else with every key that points at it.
//   - A space's: when one app's bucket is over its own caps, one of its keys goes (by the bucket's strategy). The blob
//     goes too only when nothing else points at it.
//
// A blob lives in exactly one tier. The index of keys stays in memory (and counts against the memory limit); keys
// whose blob is on disk also have a record on disk, so the disk tier survives a restart. One lock covers it all.

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
)

// keyOverhead is what a key costs in memory beyond its name: map slot, list element, struct.
const keyOverhead = 96

// recordMagic starts every key record: "P1K1", expiry (unix ns, 0 none), blob hash, space length, space, key.
const recordMagic = "P1K1"

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

// spaceLimits are one app's bucket's caps inside the store.
type spaceLimits struct {
	eviction cachev1.Eviction
	maxItems int
	maxMem   int64 // logical: every key's name and its value's full size, even when the value is shared
	ttl      time.Duration
}

type blob struct {
	hash sum
	size int64
	data []byte // nil while on disk
	disk bool
	refs map[*entry]struct{}
	el   *list.Element // in store.mem or store.disk
}

type entry struct {
	sp  *space
	key string
	b   *blob
	exp int64 // unix ns; 0 = never
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
	disk       blobList
	keyBytes   int64 // the key index, always in memory
	keys       int
	spaces     map[string]*space
	sinceSweep int
}

func newStore(lim poolLimits, log *slog.Logger) (*store, error) {
	s := &store{lim: lim, log: log, now: time.Now, blobs: map[sum]*blob{}, spaces: map[string]*space{},
		mem: blobList{order: list.New()}, disk: blobList{order: list.New()}}
	if lim.dir != "" {
		if err := s.load(); err != nil {
			return nil, fmt.Errorf("cache: disk tier %s: %w", lim.dir, err)
		}
	}
	return s, nil
}

// open returns space id with lim, creating it. An app that starts again reopens its spaces with its current limits;
// what they hold stays.
func (s *store) open(id string, lim spaceLimits) *space {
	s.mu.Lock()
	defer s.mu.Unlock()
	sp := s.spaceLocked(id)
	sp.lim = lim
	s.fitSpace(sp, nil)
	return sp
}

func (s *store) spaceLocked(id string) *space {
	sp, ok := s.spaces[id]
	if !ok {
		sp = &space{id: id, entries: map[string]*entry{}, order: list.New()}
		s.spaces[id] = sp
	}
	return sp
}

func (s *store) get(sp *space, key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := sp.entries[key]
	if !ok {
		return nil, false
	}
	if e.expired(s.now().UnixNano()) {
		s.removeEntry(e)
		return nil, false
	}
	if touches(sp.lim.eviction) {
		sp.order.MoveToFront(e.el)
	}
	b := e.b
	if !b.disk {
		if touches(s.lim.eviction) {
			s.mem.order.MoveToFront(b.el)
		}
		return bytes.Clone(b.data), true
	}
	data, err := s.promote(b)
	if err != nil {
		s.log.Warn("cache: unreadable disk entry dropped", "error", err)
		s.dropBlob(b)
		return nil, false
	}
	return bytes.Clone(data), true
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
	case shared && b.disk:
		s.writeRecord(e)
	case shared:
		if touches(s.lim.eviction) {
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
	if b.disk {
		_ = os.Remove(s.recordPath(sp.id, e.key))
	}
	if len(b.refs) == 0 {
		s.freeBlob(b)
	}
}

// freeBlob forgets b (no keys point at it any more).
func (s *store) freeBlob(b *blob) {
	delete(s.blobs, b.hash)
	if b.el == nil {
		return
	}
	if b.disk {
		s.disk.order.Remove(b.el)
		s.disk.bytes -= b.size
		_ = os.Remove(s.blobPath(b.hash))
	} else {
		s.mem.order.Remove(b.el)
		s.mem.bytes -= b.size
	}
	b.el = nil
}

// dropBlob evicts b from the store with every key that points at it.
func (s *store) dropBlob(b *blob) {
	for e := range b.refs {
		s.removeEntry(e) // the last one frees b
	}
	if len(b.refs) == 0 && s.blobs[b.hash] == b {
		s.freeBlob(b)
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

// spill moves b (in no list) to the disk tier, writing its file and its keys' records; without a disk tier, or when
// it doesn't fit, it drops b with its keys.
func (s *store) spill(b *blob) {
	if s.lim.dir == "" || over(s.lim.maxDisk, b.size) {
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
	if err := writeAtomic(s.lim.dir, s.blobPath(b.hash), b.data); err != nil {
		s.log.Warn("cache: disk write failed, entry dropped", "error", err)
		s.dropBlob(b)
		return
	}
	b.data, b.disk = nil, true
	b.el = s.disk.order.PushFront(b)
	s.disk.bytes += b.size
	for e := range b.refs {
		s.writeRecord(e)
	}
}

// promote reads b from disk and moves it to memory when it fits there (else it stays on disk, read each time).
func (s *store) promote(b *blob) ([]byte, error) {
	data, err := os.ReadFile(s.blobPath(b.hash))
	if err != nil {
		return nil, err
	}
	if sha256.Sum256(data) != b.hash {
		return nil, errors.New("blob doesn't match its hash")
	}
	if over(s.lim.maxMem, b.size+s.keyBytes) {
		if touches(s.lim.eviction) {
			s.disk.order.MoveToFront(b.el)
		}
		return data, nil
	}
	s.disk.order.Remove(b.el)
	s.disk.bytes -= b.size
	_ = os.Remove(s.blobPath(b.hash))
	for e := range b.refs {
		_ = os.Remove(s.recordPath(e.sp.id, e.key))
	}
	b.data, b.disk = data, false
	b.el = s.mem.order.PushFront(b)
	s.mem.bytes += b.size
	s.fitMem(b)
	return data, nil
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

// ---- disk ----

func (s *store) blobPath(h sum) string {
	x := h.hex()
	return filepath.Join(s.lim.dir, "blobs", x[:2], x)
}

func (s *store) recordPath(spaceID, key string) string {
	x := hex.EncodeToString(func() []byte { h := sha256.Sum256([]byte(spaceID + "\x00" + key)); return h[:] }())
	return filepath.Join(s.lim.dir, "keys", x[:2], x)
}

func (s *store) writeRecord(e *entry) {
	b := make([]byte, 0, 4+8+32+4+len(e.sp.id)+len(e.key))
	b = append(b, recordMagic...)
	b = binary.BigEndian.AppendUint64(b, uint64(e.exp))
	b = append(b, e.b.hash[:]...)
	b = binary.BigEndian.AppendUint32(b, uint32(len(e.sp.id)))
	b = append(append(b, e.sp.id...), e.key...)
	if err := writeAtomic(s.lim.dir, s.recordPath(e.sp.id, e.key), b); err != nil {
		s.log.Warn("cache: disk record write failed", "error", err) // the key just won't survive a restart
	}
}

type record struct {
	exp          int64
	hash         sum
	spaceID, key string
}

func parseRecord(b []byte) (record, error) {
	var r record
	if len(b) < 48 || string(b[:4]) != recordMagic {
		return r, errors.New("not a cache key record")
	}
	r.exp = int64(binary.BigEndian.Uint64(b[4:]))
	copy(r.hash[:], b[12:44])
	n := int(binary.BigEndian.Uint32(b[44:]))
	if n > len(b)-48 {
		return r, errors.New("short cache key record")
	}
	r.spaceID, r.key = string(b[48:48+n]), string(b[48+n:])
	return r, nil
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

// load indexes what an earlier run left on disk: key records whose blob is there (oldest blob file last), dropping
// temp files, broken or expired records, and blobs no record points at. Then it trims the tier to maxDisk.
func (s *store) load() error {
	if err := os.MkdirAll(s.lim.dir, 0o755); err != nil {
		return err
	}
	if tops, err := os.ReadDir(s.lim.dir); err == nil {
		for _, d := range tops {
			if strings.HasPrefix(d.Name(), ".tmp-") {
				_ = os.Remove(filepath.Join(s.lim.dir, d.Name()))
			}
		}
	}
	now := s.now().UnixNano()
	type found struct {
		size int64
		mod  time.Time
	}
	files := map[sum]found{}
	blobsDir := filepath.Join(s.lim.dir, "blobs")
	err := walkFiles(blobsDir, func(p string, info fs.FileInfo) {
		h, err := hex.DecodeString(filepath.Base(p))
		if err != nil || len(h) != len(sum{}) || p != s.blobPath(sum(h)) {
			_ = os.Remove(p)
			return
		}
		files[sum(h)] = found{info.Size(), info.ModTime()}
	})
	if err != nil {
		return err
	}
	used := map[sum]bool{}
	var recs []record
	err = walkFiles(filepath.Join(s.lim.dir, "keys"), func(p string, _ fs.FileInfo) {
		b, err := os.ReadFile(p)
		r, perr := parseRecord(b)
		_, have := files[r.hash]
		if err != nil || perr != nil || !have || r.exp != 0 && now >= r.exp || p != s.recordPath(r.spaceID, r.key) {
			_ = os.Remove(p)
			return
		}
		used[r.hash] = true
		recs = append(recs, r)
	})
	if err != nil {
		return err
	}
	hashes := make([]sum, 0, len(used))
	for h := range files {
		if !used[h] {
			_ = os.Remove(s.blobPath(h))
			continue
		}
		hashes = append(hashes, h)
	}
	sort.Slice(hashes, func(i, j int) bool { return files[hashes[i]].mod.Before(files[hashes[j]].mod) })
	for _, h := range hashes { // oldest first, so the newest ends at the front
		b := &blob{hash: h, size: files[h].size, disk: true, refs: map[*entry]struct{}{}}
		b.el = s.disk.order.PushFront(b)
		s.disk.bytes += b.size
		s.blobs[h] = b
	}
	for _, r := range recs {
		s.attach(&entry{sp: s.spaceLocked(r.spaceID), key: r.key, exp: r.exp}, s.blobs[r.hash])
	}
	for over(s.lim.maxDisk, s.disk.bytes) {
		s.dropBlob(victim(s.disk.order, s.lim.eviction, nil).Value.(*blob))
	}
	return nil
}

// walkFiles calls fn for every regular file under dir (a missing dir has none), removing leftover temp files.
func walkFiles(dir string, fn func(p string, info fs.FileInfo)) error {
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasPrefix(d.Name(), ".tmp-") {
			_ = os.Remove(p)
			return nil
		}
		if info, err := d.Info(); err == nil {
			fn(p, info)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
