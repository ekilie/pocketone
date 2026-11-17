package cache

// The write-through index: what each key points at on disk, so the cache comes back after a restart. It's an
// append-only file of JSON lines, one record per change:
//
//	{"op":"set","s":"inst1/pages","k":"t:ab12","v":"tpl2","h":"9f3c…","n":4096,"x":0,"q":17}
//	{"op":"del","s":"inst1/pages","k":"t:ab12","q":23}
//
// A set says (space, key, version) -> sha (its file, "n" bytes), with its expiry; a del says the key no longer points
// at a file. Every record carries a sequence number taken under the store's lock when the change happened, so records
// may reach the file in any order: replay keeps, per key, the record with the highest number. A set is appended once
// its file is written; a del as soon as the change that caused it lets go of the lock, before the call that made it
// returns. A key replaced or deleted before a crash never comes back.
//
// Replay at start rebuilds every key on disk (none in memory), drops expired ones and keys whose file is missing,
// removes files no key points at, and rewrites the index from what's left. The index is rewritten the same way
// (compacted) on the IO goroutine once it holds far more records than keys.

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

const walFile = "index.log"

const (
	opSet = "set"
	opDel = "del"
)

type walRec struct {
	Op    string `json:"op"`
	Space string `json:"s"`
	Key   string `json:"k"`
	Ver   string `json:"v,omitempty"`
	Hash  string `json:"h,omitempty"`
	Size  int64  `json:"n,omitempty"`
	Exp   int64  `json:"x,omitempty"`
	Seq   uint64 `json:"q"`
}

// compactAt: the index is compacted once it holds more than this many records, and more than four per live key.
const compactAt = 1024

type wal struct {
	path string

	// under the store's lock
	seq        uint64
	q          []walRec
	compacting bool

	mu sync.Mutex // the file
	f  *os.File
	n  int // records in the file
}

func (w *wal) next() uint64 { w.seq++; return w.seq }

func (w *wal) queue(r walRec) { w.q = append(w.q, r) }

func (w *wal) take() []walRec {
	q := w.q
	w.q = nil
	return q
}

// append writes recs to the file (off the store's lock), and queues a compaction when it has grown far past the keys.
func (w *wal) append(s *store, recs []walRec) {
	buf := make([]byte, 0, 128*len(recs))
	for _, r := range recs {
		b, _ := json.Marshal(r)
		buf = append(append(buf, b...), '\n')
	}
	w.mu.Lock()
	if w.f == nil {
		w.mu.Unlock()
		return
	}
	if _, err := w.f.Write(buf); err != nil {
		s.log.Warn("cache: index write failed", "error", err)
	}
	w.n += len(recs)
	n := w.n
	w.mu.Unlock()
	if n <= compactAt {
		return
	}
	s.mu.Lock()
	if !w.compacting && n > 4*s.keys+compactAt {
		w.compacting = true
		s.io.compact(s)
	}
	s.mu.Unlock()
}

// compact rewrites the index from the store's state: one set per key whose file is written.
func (w *wal) compact(s *store) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	s.mu.Lock()
	w.q = nil // what's queued is already part of the state below
	w.compacting = false
	recs := s.snapshot()
	s.mu.Unlock()
	return w.rewrite(recs)
}

// snapshot is a set record for every key whose file is written, under the lock.
func (s *store) snapshot() []walRec {
	var recs []walRec
	for _, sp := range s.spaces {
		for _, e := range sp.entries {
			if e.disk != nil && e.disk.written && !e.writing {
				b := e.disk
				recs = append(recs, walRec{Op: opSet, Space: sp.id, Key: e.key, Ver: e.ver, Hash: b.hash.hex(), Size: b.size, Exp: e.exp, Seq: e.seq})
			}
		}
	}
	slices.SortFunc(recs, func(a, b walRec) int { return int(a.Seq - b.Seq) })
	return recs
}

// rewrite replaces the index with recs and reopens it for appending; w.mu is held (or w isn't shared yet).
func (w *wal) rewrite(recs []walRec) error {
	f, err := os.CreateTemp(filepath.Dir(w.path), ".tmp-*")
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(f)
	for _, r := range recs {
		b, _ := json.Marshal(r)
		bw.Write(b)
		bw.WriteByte('\n')
	}
	err = errors.Join(bw.Flush(), f.Sync(), f.Close())
	if err == nil {
		err = os.Rename(f.Name(), w.path)
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	if w.f != nil {
		_ = w.f.Close()
	}
	w.f, err = os.OpenFile(w.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	w.n = len(recs)
	return err
}

func (w *wal) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
}

// replay rebuilds the store from the disk tier's index (called by newStore, before the store is shared).
func (s *store) replay() error {
	dir := s.lim.dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	w := &wal{path: filepath.Join(dir, walFile)}
	latest := map[[2]string]walRec{}
	f, err := os.Open(w.path)
	switch {
	case err == nil:
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			var r walRec
			if json.Unmarshal(sc.Bytes(), &r) != nil {
				continue // a torn last line
			}
			w.seq = max(w.seq, r.Seq)
			k := [2]string{r.Space, r.Key}
			if old, ok := latest[k]; !ok || r.Seq > old.Seq {
				latest[k] = r
			}
		}
		f.Close()
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}

	now := s.now().UnixNano()
	live := make([]walRec, 0, len(latest))
	for _, r := range latest {
		if r.Op == opSet && (r.Exp == 0 || now < r.Exp) {
			live = append(live, r)
		}
	}
	slices.SortFunc(live, func(a, b walRec) int { return int(a.Seq - b.Seq) }) // oldest first: newest end up in front
	for _, r := range live {
		h, ok := parseSum(r.Hash)
		if !ok {
			continue
		}
		b := s.blobs[h]
		if b == nil {
			st, err := os.Stat(s.blobPath(h))
			if err != nil || st.Size() != r.Size {
				continue
			}
			b = &blob{hash: h, size: r.Size, written: true, refs: map[*entry]struct{}{}}
			s.blobs[h] = b
			b.el = s.disk.PushFront(b)
			s.diskBytes += b.size
		} else {
			s.disk.MoveToFront(b.el)
		}
		e := &entry{sp: s.space(r.Space), key: r.Key, ver: r.Ver, exp: r.Exp, seq: r.Seq}
		s.attach(e)
		s.attachDisk(e, b)
	}
	for over(s.lim.maxDisk, s.diskBytes) {
		el := victim(s.disk, s.lim.eviction, nil)
		if el == nil {
			break
		}
		s.dropBlob(el.Value.(*blob))
	}
	s.io.flush() // removals queued by dropping
	if err := prune(dir, s.blobs); err != nil {
		return err
	}
	if err := w.rewrite(s.snapshot()); err != nil {
		return err
	}
	s.wal = w
	return nil
}
