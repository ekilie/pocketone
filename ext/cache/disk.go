package cache

// The disk tier: files named by the SHA-256 of their bytes, shared by every key that points at them, written and
// removed off the lock by one IO goroutine in the order the lock queued them.

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func (s *store) blobPath(h sum) string {
	x := h.hex()
	return filepath.Join(s.lim.dir, x[:2], x)
}

// attachDisk points e at file b.
func (s *store) attachDisk(e *entry, b *blob) {
	e.disk = b
	b.refs[e] = struct{}{}
	e.del = e.sp.disk.PushFront(e)
	e.sp.diskBytes += b.size
}

// detachDisk lets go of e's file, freeing it when no other key points at it.
func (s *store) detachDisk(e *entry) {
	b := e.disk
	delete(b.refs, e)
	e.sp.disk.Remove(e.del)
	e.sp.diskBytes -= b.size
	e.disk, e.del = nil, nil
	if len(b.refs) == 0 {
		s.freeBlob(b)
	}
}

// freeBlob forgets b and queues its file's removal.
func (s *store) freeBlob(b *blob) {
	if s.blobs[b.hash] == b {
		delete(s.blobs, b.hash)
	}
	if b.el == nil {
		return
	}
	s.disk.Remove(b.el)
	s.diskBytes -= b.size
	b.el = nil
	s.io.remove(s, b.hash)
}

// dropBlob evicts b: keys that had only the file go; keys still in memory keep their value without it.
func (s *store) dropBlob(b *blob) {
	refs := make([]*entry, 0, len(b.refs))
	for e := range b.refs {
		refs = append(refs, e)
	}
	for _, e := range refs {
		if !e.inMem {
			s.removeEntry(e)
			continue
		}
		if s.wal != nil {
			s.wal.queue(walRec{Op: opDel, Space: e.sp.id, Key: e.key, Seq: s.wal.next()}) // no longer kept across a restart
		}
		e.writing = false
		s.detachDisk(e)
	}
	if s.blobs[b.hash] == b || b.el != nil {
		s.freeBlob(b)
	}
}

// writeDone finishes e's write: its file is there, so a value held only for it goes.
func (s *store) writeDone(e *entry) {
	e.writing = false
	if !e.inMem {
		s.pending -= e.size
		e.v = nil
	}
	if s.wal != nil {
		b := e.disk
		s.wal.queue(walRec{Op: opSet, Space: e.sp.id, Key: e.key, Ver: e.ver, Hash: b.hash.hex(), Size: b.size, Exp: e.exp, Seq: e.seq})
	}
}

// writeFailed gives up on e's file: a value held only for it goes with its key.
func (s *store) writeFailed(e *entry) {
	e.writing = false
	if !e.inMem {
		s.removeEntry(e)
	}
}

// runWrite is the IO goroutine writing what a key holds now (its slot's latest Set): marshaled off the lock, then
// onto an existing file with the same bytes, or a new one.
func (s *store) runWrite(sl slot) {
	s.mu.Lock()
	e := sl.sp.entries[sl.key]
	if e == nil || !e.writing {
		s.unlock()
		return
	}
	v := e.v
	s.unlock()

	data, err := v.bytes()
	h := sum(sha256.Sum256(data))

	s.mu.Lock()
	if e.dead || !e.writing {
		s.unlock()
		return
	}
	if err != nil {
		s.log.Warn("cache: marshal failed, entry not written", "error", err)
		s.writeFailed(e)
		s.unlock()
		return
	}
	if b := s.blobs[h]; b != nil { // the same bytes are on disk already (or on their way)
		s.attachDisk(e, b)
		if b.written {
			s.writeDone(e)
		}
		s.fitSpace(e.sp, e)
		s.unlock()
		return
	}
	size := int64(len(data))
	if over(s.lim.maxDisk, size) || over(e.sp.lim.maxDisk, size) {
		s.writeFailed(e)
		s.unlock()
		return
	}
	for over(s.lim.maxDisk, s.diskBytes+size) {
		el := victim(s.disk, s.lim.eviction, nil)
		if el == nil {
			break
		}
		s.dropBlob(el.Value.(*blob))
	}
	b := &blob{hash: h, size: size, refs: map[*entry]struct{}{}}
	s.blobs[h] = b
	b.el = s.disk.PushFront(b)
	s.diskBytes += size
	s.attachDisk(e, b)
	s.fitSpace(e.sp, e)
	path := s.blobPath(h)
	s.unlock()

	werr := writeAtomic(s.lim.dir, path, data)

	s.mu.Lock()
	defer s.unlock()
	if s.blobs[h] != b {
		return // freed while written: its removal is queued after this write
	}
	if werr != nil {
		s.log.Warn("cache: disk write failed, entry dropped", "error", werr)
		s.dropBlob(b)
		return
	}
	b.written = true
	for r := range b.refs {
		if r.writing {
			s.writeDone(r)
		}
	}
}

// runRemove removes file h, unless a write queued before this removal (its hash known only when it ran) has made a
// new blob of the same bytes since: that file is live again. Only this goroutine creates blobs, so nothing can between
// the check and the removal.
func (s *store) runRemove(h sum) {
	s.mu.Lock()
	live := s.blobs[h] != nil
	s.unlock()
	if !live {
		_ = os.Remove(s.blobPath(h))
	}
}

// ---- the IO goroutine ----

// ioQueue runs file writes, removals and index compactions in the order they were queued (under the store's lock), on
// one goroutine that runs only while there is work. Writes are slotted: at most one per key waits in the queue, and it
// writes whatever the key holds when it runs, so a key Set many times before the queue reaches it is marshaled and
// written once, with its latest value.
type ioQueue struct {
	mu       sync.Mutex
	cond     *sync.Cond
	ops      []ioOp
	slots    map[slot]bool // writes waiting in ops
	running  bool
	beforeOp func() // tests: called before each op, off every lock
}

// slot is one key's place in the write queue.
type slot struct {
	sp  *space
	key string
}

type ioOp struct {
	write   *slot // write what this key holds
	remove  *sum  // or remove this file
	compact bool  // or compact the write-through index
}

// write queues e's key for writing, unless a write of it is already waiting: that one writes e's value.
func (q *ioQueue) write(s *store, e *entry) { q.push(s, ioOp{write: &slot{e.sp, e.key}}) }
func (q *ioQueue) remove(s *store, h sum)   { q.push(s, ioOp{remove: &h}) }
func (q *ioQueue) compact(s *store)         { q.push(s, ioOp{compact: true}) }

func (q *ioQueue) push(s *store, op ioOp) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if op.write != nil {
		if q.slots[*op.write] {
			return
		}
		if q.slots == nil {
			q.slots = map[slot]bool{}
		}
		q.slots[*op.write] = true
	}
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
		if op.write != nil {
			delete(q.slots, *op.write) // a Set from now on queues a new write
		}
		hook := q.beforeOp
		q.mu.Unlock()

		if hook != nil {
			hook()
		}
		switch {
		case op.write != nil:
			s.runWrite(*op.write)
		case op.compact:
			if err := s.wal.compact(s); err != nil {
				s.log.Warn("cache: index compaction failed", "error", err)
			}
		default:
			s.runRemove(*op.remove)
		}
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

// wipe empties the disk tier's folder: only what the store writes there (two-hex-digit folders of blob files, temp
// files, the index), so a disk_path pointed at the wrong place loses nothing else.
func wipe(dir string) error { return prune(dir, nil) }

// prune removes from the disk tier's folder every file the store wrote that keep doesn't hold (and, with keep nil, the
// index too).
func prune(dir string, keep map[sum]*blob) error {
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
		case d.Name() == walFile && !d.IsDir() && keep == nil:
			_ = os.Remove(p)
		case d.IsDir() && len(d.Name()) == 2 && isHex(d.Name()):
			files, err := os.ReadDir(p)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			for _, f := range files {
				if f.IsDir() || len(f.Name()) != 2*sha256.Size || !isHex(f.Name()) || f.Name()[:2] != d.Name() {
					continue
				}
				if h, ok := parseSum(f.Name()); ok && keep[h] != nil {
					continue
				}
				_ = os.Remove(filepath.Join(p, f.Name()))
			}
			_ = os.Remove(p) // only if now empty
		}
	}
	return nil
}

func parseSum(x string) (sum, bool) {
	var h sum
	if len(x) != 2*len(h) || !isHex(x) {
		return h, false
	}
	for i := range h {
		h[i] = unhex(x[2*i])<<4 | unhex(x[2*i+1])
	}
	return h, true
}

func unhex(c byte) byte {
	if c <= '9' {
		return c - '0'
	}
	return c - 'a' + 10
}

func isHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
