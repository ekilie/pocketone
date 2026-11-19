package engine

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Threads keep a conversation's model state between calls: the keys and values of every token the model read and
// wrote, so the next turn reads only what's new (the user's message), not the whole conversation again. They're a
// cache, not storage: a call continues from a thread's state as far as its prompt starts the same way, and reads the
// rest. The app keeps the conversation itself.
//
// A thread lives in memory (LoadOptions.ThreadMemory, least recently used first) and, when the call names a folder
// (GenerateOptions.ThreadDir), on disk at half precision, so it outlives memory and restarts. Each folder has its own
// byte budget (GenerateOptions.ThreadDisk), least recently used files first. Files name the model they belong to and
// are ignored by another.

// threadKey is a thread's identity: whose it is (the cache scope) and its id.
func threadKey(scope, id string) string { return scope + "\x00" + id }

type threadEntry struct {
	scope, id string
	e         *prefixEntry
	bytes     int64
	seq       uint64
	used      time.Time
}

type threadStore struct {
	mu      sync.Mutex
	budget  int64 // bytes in memory; <= 0: memory off (disk only)
	used    int64
	seq     uint64
	mem     map[string]*threadEntry
	diskMu  sync.Mutex     // one disk writer or evictor at a time
	writing sync.WaitGroup // disk writes in flight
}

// ThreadInfo describes a kept thread.
type ThreadInfo struct {
	ID     string
	Tokens int
	Bytes  int64     // in memory (full precision) or on disk (half)
	Where  string    // "memory" or "disk"
	UsedAt time.Time // last call that used or wrote it
}

func (m *Model) threadFile(dir, scope, id string) string {
	mh := sha256.Sum256([]byte(m.id))
	th := sha256.Sum256([]byte(threadKey(scope, id)))
	return filepath.Join(dir, hex.EncodeToString(mh[:6]), hex.EncodeToString(th[:16])+".kv")
}

// lookupThread finds the thread's state, in memory or on disk, and how many of ids it covers (always leaving the
// last token of ids to compute).
func (m *Model) lookupThread(scope, id, dir string, ids []int) (*prefixEntry, int) {
	t := &m.threads
	key := threadKey(scope, id)
	t.mu.Lock()
	te := t.mem[key]
	if te != nil {
		t.seq++
		te.seq, te.used = t.seq, time.Now()
	}
	t.mu.Unlock()
	if te == nil && dir != "" {
		e, err := m.readThread(m.threadFile(dir, scope, id), scope, id)
		if err != nil {
			return nil, 0
		}
		te = m.keepThread(scope, id, e)
		now := time.Now()
		_ = os.Chtimes(m.threadFile(dir, scope, id), now, now) // recently used: evicted last
	}
	if te == nil {
		return nil, 0
	}
	l := 0
	for l < len(te.e.tokens) && l < len(ids)-1 && te.e.tokens[l] == ids[l] {
		l++
	}
	if l < prefixMin {
		return nil, 0
	}
	return te.e, l
}

// keepThread puts e in memory as the thread's state (replacing the old one) and evicts past the budget. It returns
// the entry even when it doesn't fit in memory, for the call at hand.
func (m *Model) keepThread(scope, id string, e *prefixEntry) *threadEntry {
	t := &m.threads
	size := int64(len(e.tokens)) * m.tokenBytes()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	te := &threadEntry{scope: scope, id: id, e: e, bytes: size, seq: t.seq, used: time.Now()}
	key := threadKey(scope, id)
	if old := t.mem[key]; old != nil {
		t.used -= old.bytes
		delete(t.mem, key)
	}
	if size > t.budget {
		return te // too big for memory: disk only
	}
	for t.used+size > t.budget {
		var oldKey string
		var oldest *threadEntry
		for k, c := range t.mem {
			if oldest == nil || c.seq < oldest.seq {
				oldKey, oldest = k, c
			}
		}
		t.used -= oldest.bytes
		delete(t.mem, oldKey)
	}
	if t.mem == nil {
		t.mem = map[string]*threadEntry{}
	}
	t.mem[key] = te
	t.used += size
	return te
}

// storeThread keeps the state of tokens (all computed in s) as the thread's, in memory and, with a folder, on disk.
func (m *Model) storeThread(o GenerateOptions, s *state, tokens []int) {
	n := len(tokens)
	if n < prefixMin {
		return
	}
	kvd := m.hp.kvHeads * m.hp.headDim
	e := &prefixEntry{scope: o.CacheScope, tokens: append([]int(nil), tokens...), k: make([][]float32, len(s.kc)), v: make([][]float32, len(s.vc))}
	for li := range s.kc {
		k := make([]float32, kvd*n)
		for i := 0; i < kvd; i++ {
			copy(k[i*n:(i+1)*n], s.kc[li][i*s.ctxP:i*s.ctxP+n])
		}
		e.k[li] = k
		e.v[li] = append([]float32(nil), s.vc[li][:n*kvd]...)
	}
	m.keepThread(o.CacheScope, o.Thread, e)
	if o.ThreadDir == "" {
		return
	}
	path := m.threadFile(o.ThreadDir, o.CacheScope, o.Thread)
	m.threads.writing.Add(1)
	go func() { // off the answer's path: half a second for a long thread
		defer m.threads.writing.Done()
		m.threads.diskMu.Lock()
		defer m.threads.diskMu.Unlock()
		if err := m.writeThread(path, o.CacheScope, o.Thread, e); err != nil {
			return // a cache: the next call reads the conversation again
		}
		evictDir(o.ThreadDir, o.ThreadDisk)
	}()
}

const threadMagic = "P1TH0001"

// writeThread saves e at half precision: header (magic, model, scope, id, layers, kvd, n), the tokens, then each
// layer's keys and values.
func (m *Model) writeThread(path, scope, id string, e *prefixEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	w := bufio.NewWriterSize(tmp, 1<<20)
	kvd := m.hp.kvHeads * m.hp.headDim
	n := len(e.tokens)
	w.WriteString(threadMagic)
	for _, s := range []string{m.id, scope, id} {
		binary.Write(w, binary.LittleEndian, uint16(len(s)))
		w.WriteString(s)
	}
	binary.Write(w, binary.LittleEndian, [3]uint32{uint32(len(e.k)), uint32(kvd), uint32(n)})
	for _, t := range e.tokens {
		binary.Write(w, binary.LittleEndian, int32(t))
	}
	buf := make([]byte, 2*kvd*n)
	for li := range e.k {
		for _, part := range [][]float32{e.k[li], e.v[li]} {
			for i, f := range part {
				binary.LittleEndian.PutUint16(buf[2*i:], f32ToF16(f))
			}
			w.Write(buf[:2*len(part)])
		}
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

var errThreadFile = errors.New("engine: not this model's thread file")

type threadHeader struct {
	model, scope, id string
	layers, kvd, n   int
}

func readThreadHeader(r io.Reader) (threadHeader, error) {
	var h threadHeader
	magic := make([]byte, len(threadMagic))
	if _, err := io.ReadFull(r, magic); err != nil || string(magic) != threadMagic {
		return h, errThreadFile
	}
	strs := make([]string, 3)
	for i := range strs {
		var l uint16
		if err := binary.Read(r, binary.LittleEndian, &l); err != nil {
			return h, err
		}
		b := make([]byte, l)
		if _, err := io.ReadFull(r, b); err != nil {
			return h, err
		}
		strs[i] = string(b)
	}
	var dims [3]uint32
	if err := binary.Read(r, binary.LittleEndian, &dims); err != nil {
		return h, err
	}
	return threadHeader{strs[0], strs[1], strs[2], int(dims[0]), int(dims[1]), int(dims[2])}, nil
}

func (m *Model) readThread(path, scope, id string) (*prefixEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	h, err := readThreadHeader(r)
	if err != nil {
		return nil, err
	}
	kvd := m.hp.kvHeads * m.hp.headDim
	if h.model != m.id || h.scope != scope || h.id != id || h.layers != m.hp.layers || h.kvd != kvd || h.n <= 0 || h.n > m.hp.ctx {
		return nil, errThreadFile
	}
	e := &prefixEntry{scope: scope, tokens: make([]int, h.n), k: make([][]float32, h.layers), v: make([][]float32, h.layers)}
	tb := make([]int32, h.n)
	if err := binary.Read(r, binary.LittleEndian, tb); err != nil {
		return nil, err
	}
	for i, t := range tb {
		e.tokens[i] = int(t)
	}
	buf := make([]byte, 2*kvd*h.n)
	for li := 0; li < h.layers; li++ {
		for part := 0; part < 2; part++ {
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}
			out := make([]float32, kvd*h.n)
			for i := range out {
				out[i] = f16Table[binary.LittleEndian.Uint16(buf[2*i:])]
			}
			if part == 0 {
				e.k[li] = out
			} else {
				e.v[li] = out
			}
		}
	}
	return e, nil
}

// evictDir removes the least recently used thread files under dir until they take at most budget bytes (<= 0: no
// limit). Files of other models count too, and go first when they're older.
func evictDir(dir string, budget int64) {
	if budget <= 0 {
		return
	}
	type file struct {
		path string
		size int64
		mod  time.Time
	}
	var files []file
	var total int64
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".kv") {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files = append(files, file{p, info.Size(), info.ModTime()})
			total += info.Size()
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for _, f := range files {
		if total <= budget {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}

// Threads lists the threads kept for scope: in memory, and in dir when given.
func (m *Model) Threads(scope, dir string) []ThreadInfo {
	seen := map[string]bool{}
	var out []ThreadInfo
	t := &m.threads
	t.mu.Lock()
	for _, te := range t.mem {
		if te.scope == scope {
			seen[te.id] = true
			out = append(out, ThreadInfo{ID: te.id, Tokens: len(te.e.tokens), Bytes: te.bytes, Where: "memory", UsedAt: te.used})
		}
	}
	t.mu.Unlock()
	if dir != "" {
		mh := sha256.Sum256([]byte(m.id))
		entries, _ := os.ReadDir(filepath.Join(dir, hex.EncodeToString(mh[:6])))
		for _, de := range entries {
			if !strings.HasSuffix(de.Name(), ".kv") {
				continue
			}
			p := filepath.Join(dir, hex.EncodeToString(mh[:6]), de.Name())
			f, err := os.Open(p)
			if err != nil {
				continue
			}
			h, err := readThreadHeader(bufio.NewReader(f))
			f.Close()
			if err != nil || h.scope != scope || seen[h.id] {
				continue
			}
			info, _ := de.Info()
			out = append(out, ThreadInfo{ID: h.id, Tokens: h.n, Bytes: info.Size(), Where: "disk", UsedAt: info.ModTime()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UsedAt.After(out[j].UsedAt) })
	return out
}

// ForgetThread drops a thread's state from memory and, when dir is given, from disk.
func (m *Model) ForgetThread(scope, id, dir string) {
	t := &m.threads
	t.mu.Lock()
	if te := t.mem[threadKey(scope, id)]; te != nil {
		t.used -= te.bytes
		delete(t.mem, threadKey(scope, id))
	}
	t.mu.Unlock()
	if dir != "" {
		t.diskMu.Lock()
		_ = os.Remove(m.threadFile(dir, scope, id))
		t.diskMu.Unlock()
	}
}

// FlushThreads waits for thread files being written.
func (m *Model) FlushThreads() { m.threads.writing.Wait() }

// ThreadTokenBytes is what one token of a thread takes in memory (full precision); on disk it's half.
func (m *Model) ThreadTokenBytes() int64 { return m.tokenBytes() }

// String is for logs.
func (t ThreadInfo) String() string {
	return fmt.Sprintf("%s: %d tokens, %d bytes in %s", t.ID, t.Tokens, t.Bytes, t.Where)
}
