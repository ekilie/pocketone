package cache

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

// entryOverhead is what an entry costs in memory beyond its key and value: the map slot, list element and struct.
const entryOverhead = 64

// diskMagic starts every disk entry: "P1C1", expiry (unix ns, 0 none), key length, key, value.
const diskMagic = "P1C1"

const diskHeader = len(diskMagic) + 8 + 4

// limits are a bucket's options with every default filled in.
type limits struct {
	eviction cachev1.Eviction
	maxItems int
	maxMem   int64
	ttl      time.Duration
	dir      string // "" = memory only
	maxDisk  int64
}

// entry is one key. In memory it holds its value; on disk only its size (the value is in its file).
type entry struct {
	key  string
	val  []byte
	size int64
	exp  int64 // unix ns; 0 = never
}

func (e *entry) expired(now int64) bool { return e.exp != 0 && now >= e.exp }

// tier is one level of a bucket: its entries in use order, newest at the front.
type tier struct {
	items map[string]*list.Element
	order *list.List
	bytes int64
}

func newTier() tier { return tier{items: map[string]*list.Element{}, order: list.New()} }

func (t *tier) remove(el *list.Element) *entry {
	e := t.order.Remove(el).(*entry)
	delete(t.items, e.key)
	t.bytes -= e.size
	return e
}

// victim is the entry the eviction strategy drops next.
func (t *tier) victim(ev cachev1.Eviction) *list.Element {
	if ev == cachev1.MRU || ev == cachev1.LIFO {
		return t.order.Front()
	}
	return t.order.Back()
}

// bucket is ext/cache's cachev1.Bucket: a memory tier and an optional disk tier. Entries live in exactly one tier:
// memory evicts to disk, a disk hit moves back to memory. One lock covers both, so a delete can never race a spill.
type bucket struct {
	name string
	lim  limits
	log  *slog.Logger
	now  func() time.Time

	mu        sync.Mutex
	mem       tier
	disk      *tier // nil = memory only
	sinceScan int   // writes since the last expiry sweep
}

var _ cachev1.Bucket = (*bucket)(nil)

func newBucket(name string, lim limits, log *slog.Logger) (*bucket, error) {
	b := &bucket{name: name, lim: lim, log: log, now: time.Now, mem: newTier()}
	if lim.dir != "" {
		d := newTier()
		b.disk = &d
		if err := b.loadDisk(); err != nil {
			return nil, fmt.Errorf("cache: bucket %s: %w", name, err)
		}
	}
	return b, nil
}

// Get implements cachev1.Bucket.
func (b *bucket) Get(_ context.Context, key string) ([]byte, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now().UnixNano()
	if el, ok := b.mem.items[key]; ok {
		e := el.Value.(*entry)
		if e.expired(now) {
			b.mem.remove(el)
			return nil, false, nil
		}
		if b.lim.eviction == cachev1.LRU || b.lim.eviction == cachev1.MRU {
			b.mem.order.MoveToFront(el)
		}
		return append([]byte(nil), e.val...), true, nil
	}
	if b.disk == nil {
		return nil, false, nil
	}
	el, ok := b.disk.items[key]
	if !ok {
		return nil, false, nil
	}
	e := b.disk.remove(el)
	path := b.path(key)
	if e.expired(now) {
		_ = os.Remove(path)
		return nil, false, nil
	}
	val, exp, err := readEntry(path, key)
	_ = os.Remove(path) // it moves back to memory (or is unreadable)
	if err != nil {
		b.log.Warn("cache: unreadable disk entry dropped", "bucket", b.name, "error", err)
		return nil, false, nil
	}
	b.putMem(&entry{key: key, val: val, exp: exp}, now)
	return append([]byte(nil), val...), true, nil
}

// Set implements cachev1.Bucket.
func (b *bucket) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deleteLocked(key)
	if ttl < 0 {
		return nil
	}
	if ttl == 0 {
		ttl = b.lim.ttl
	}
	now := b.now().UnixNano()
	e := &entry{key: key, val: append([]byte(nil), value...)}
	if ttl > 0 {
		e.exp = now + int64(ttl)
	}
	b.sinceScan++
	if b.sinceScan > b.mem.order.Len()+16 { // amortised: one sweep per len(items) writes
		b.sweep(now)
	}
	b.putMem(e, now)
	return nil
}

// Delete implements cachev1.Bucket.
func (b *bucket) Delete(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deleteLocked(key)
	return nil
}

// Clear implements cachev1.Bucket.
func (b *bucket) Clear(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mem = newTier()
	if b.disk == nil {
		return nil
	}
	*b.disk = newTier()
	if err := os.RemoveAll(b.lim.dir); err != nil {
		return err
	}
	return os.MkdirAll(b.lim.dir, 0o755)
}

func (b *bucket) deleteLocked(key string) {
	if el, ok := b.mem.items[key]; ok {
		b.mem.remove(el)
	}
	if b.disk != nil {
		if el, ok := b.disk.items[key]; ok {
			b.disk.remove(el)
			_ = os.Remove(b.path(key))
		}
	}
}

// sweep drops expired entries from memory (disk entries expire when read or evicted).
func (b *bucket) sweep(now int64) {
	b.sinceScan = 0
	for el := b.mem.order.Front(); el != nil; {
		next := el.Next()
		if el.Value.(*entry).expired(now) {
			b.mem.remove(el)
		}
		el = next
	}
}

// putMem adds e to memory, evicting (to disk, when there is one) until it fits. An entry larger than the whole
// memory tier goes straight to disk, or is dropped.
func (b *bucket) putMem(e *entry, now int64) {
	e.size = int64(len(e.key)+len(e.val)) + entryOverhead
	if e.size > b.lim.maxMem {
		b.putDisk(e, now)
		return
	}
	for b.mem.order.Len() > 0 && (b.mem.bytes+e.size > b.lim.maxMem || b.lim.maxItems > 0 && b.mem.order.Len() >= b.lim.maxItems) {
		v := b.mem.remove(b.mem.victim(b.lim.eviction))
		if !v.expired(now) {
			b.putDisk(v, now)
		}
	}
	b.mem.items[e.key] = b.mem.order.PushFront(e)
	b.mem.bytes += e.size
}

// putDisk writes e to the disk tier, evicting disk entries until it fits; without a disk tier it drops e.
func (b *bucket) putDisk(e *entry, now int64) {
	if b.disk == nil {
		return
	}
	size := int64(diskHeader + len(e.key) + len(e.val))
	if size > b.lim.maxDisk {
		return
	}
	for b.disk.order.Len() > 0 && b.disk.bytes+size > b.lim.maxDisk {
		v := b.disk.remove(b.disk.victim(b.lim.eviction))
		_ = os.Remove(b.path(v.key))
	}
	if err := writeEntry(b.lim.dir, b.path(e.key), e); err != nil {
		b.log.Warn("cache: disk write failed, entry dropped", "bucket", b.name, "error", err)
		return
	}
	b.disk.items[e.key] = b.disk.order.PushFront(&entry{key: e.key, size: size, exp: e.exp})
	b.disk.bytes += size
}

// path is key's file: <dir>/<first 2 hex of sha256>/<sha256 hex>.
func (b *bucket) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	h := hex.EncodeToString(sum[:])
	return filepath.Join(b.lim.dir, h[:2], h)
}

func writeEntry(dir, path string, e *entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	buf := make([]byte, diskHeader, diskHeader+len(e.key)+len(e.val))
	copy(buf, diskMagic)
	binary.BigEndian.PutUint64(buf[4:], uint64(e.exp))
	binary.BigEndian.PutUint32(buf[12:], uint32(len(e.key)))
	buf = append(append(buf, e.key...), e.val...)
	_, werr := f.Write(buf)
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	return nil
}

// readHeader reads an entry file's expiry and key.
func readHeader(r io.Reader) (exp int64, key string, err error) {
	h := make([]byte, diskHeader)
	if _, err := io.ReadFull(r, h); err != nil {
		return 0, "", err
	}
	if string(h[:4]) != diskMagic {
		return 0, "", errors.New("not a cache entry")
	}
	k := make([]byte, binary.BigEndian.Uint32(h[12:]))
	if _, err := io.ReadFull(r, k); err != nil {
		return 0, "", err
	}
	return int64(binary.BigEndian.Uint64(h[4:])), string(k), nil
}

// readEntry reads key's value and expiry from its file.
func readEntry(path, key string) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	exp, k, err := readHeader(f)
	if err != nil {
		return nil, 0, err
	}
	if k != key {
		return nil, 0, errors.New("key mismatch")
	}
	val, err := io.ReadAll(f)
	return val, exp, err
}

// loadDisk indexes what an earlier run left in the disk tier (oldest file last), dropping temp files, broken and
// expired entries, then trims it to maxDisk.
func (b *bucket) loadDisk() error {
	if err := os.MkdirAll(b.lim.dir, 0o755); err != nil {
		return err
	}
	type found struct {
		e   *entry
		mod time.Time
	}
	var all []found
	now := b.now().UnixNano()
	err := filepath.WalkDir(b.lim.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasPrefix(d.Name(), ".tmp-") {
			_ = os.Remove(p)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		exp, key, herr := readHeader(f)
		f.Close()
		if herr != nil || p != b.path(key) || (exp != 0 && now >= exp) {
			_ = os.Remove(p)
			return nil
		}
		all = append(all, found{&entry{key: key, size: info.Size(), exp: exp}, info.ModTime()})
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].mod.Before(all[j].mod) })
	for _, f := range all { // oldest first, so the newest ends at the front
		b.disk.items[f.e.key] = b.disk.order.PushFront(f.e)
		b.disk.bytes += f.e.size
	}
	for b.disk.order.Len() > 0 && b.disk.bytes > b.lim.maxDisk {
		v := b.disk.remove(b.disk.victim(b.lim.eviction))
		_ = os.Remove(b.path(v.key))
	}
	return nil
}
