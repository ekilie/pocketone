package cache

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
)

func wbLimits(dir string) poolLimits {
	return poolLimits{eviction: cachev1.LRU, maxMem: 1 << 20, dir: dir, maxDisk: 1 << 20, writeBehind: true}
}

// reopen closes s (cleanly, or as after a crash) and starts a store on the same folder.
func reopen(t *testing.T, s *store, crash bool) *store {
	t.Helper()
	if crash {
		s.flush()
		s.idx.close()
	} else {
		s.close()
	}
	s2, err := newStore(wbLimits(filepath.Dir(s.lim.dir)), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s2.close)
	return s2
}

func TestWriteThroughSurvivesRestart(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(fmt.Sprintf("crash=%v", crash), func(t *testing.T) {
			dir := t.TempDir()
			s := newTestStore(t, wbLimits(dir))
			a, b := s.open("inst1/x", lru(0)), s.open("inst2/y", lru(0))
			put(s, a, "bytes", "kept")
			s.setItem(a, "item", &thing{s: "tpl", ver: "t1"}, 0)
			put(s, b, "same", "kept") // the same bytes: one file
			s.setItem(a, "live", &live{}, 0)
			put(s, a, "replaced", "old")
			put(s, a, "replaced", "new")
			put(s, a, "deleted", "gone")
			s.del(a, "deleted")
			s.flush()
			if len(s.blobs) != 3 { // kept (for both keys), tpl, new; "old" and "gone" were let go of
				t.Fatalf("%d files before restart", len(s.blobs))
			}

			s = reopen(t, s, crash)
			a, b = s.open("inst1/x", lru(0)), s.open("inst2/y", lru(0))
			if val(s, a, "bytes") != "kept" || val(s, b, "same") != "kept" || val(s, a, "replaced") != "new" {
				t.Fatal("values lost or wrong after restart")
			}
			if has(s, a, "deleted") {
				t.Fatal("a deleted key came back")
			}
			if _, ok, _ := s.getItem(a, "live", thingDecoder); ok {
				t.Fatal("a memory-only item came back")
			}
			if _, ok, _ := s.getItem(a, "item", versionDecoder("t2")); ok {
				t.Fatal("a t2 decoder read t1 bytes after restart")
			}
			it, ok, _ := s.getItem(a, "item", versionDecoder("t1"))
			if !ok || it.(*thing).s != "tpl" {
				t.Fatal("an item didn't come back after restart")
			}
			if err := checkFiles(s); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Under write-behind, memory eviction only drops the in-memory copy: the file is already there.
func TestWriteThroughEvictionKeepsFile(t *testing.T) {
	s := newTestStore(t, wbLimits(t.TempDir()))
	sp := s.open("/x", room(2, 10))
	v := &thing{s: "first"}
	s.setItem(sp, "a", v, 0)
	s.flush()
	if v.marshals.Load() != 1 || sp.entries["a"].disk == nil {
		t.Fatal("write-behind should write on Set")
	}
	s.setItem(sp, "b", &thing{s: "second"}, 0) // a leaves memory
	s.flush()
	if e := sp.entries["a"]; e.inMem || e.disk == nil {
		t.Fatal("a should be on disk only")
	}
	if v.marshals.Load() != 1 {
		t.Fatal("eviction marshaled again")
	}
	if getThing(t, s, sp, "a").s != "first" || sp.entries["a"].disk == nil {
		t.Fatal("read back keeps the file under write-behind")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}

func TestReplayDropsMissingAndStray(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t, wbLimits(dir))
	sp := s.open("/x", lru(0))
	put(s, sp, "lost", "its file goes missing")
	put(s, sp, "kept", "fine")
	s.flush()
	os.Remove(s.blobPath(sum(sha256Of("its file goes missing"))))
	stray := filepath.Join(s.lim.dir, "ab", "ab"+fmt.Sprintf("%062d", 0))
	os.MkdirAll(filepath.Dir(stray), 0o755)
	os.WriteFile(stray, []byte("no key"), 0o644)

	s = reopen(t, s, false)
	sp = s.open("/x", lru(0))
	if has(s, sp, "lost") {
		t.Fatal("a key whose file is missing came back")
	}
	if val(s, sp, "kept") != "fine" {
		t.Fatal("a key with its file was lost")
	}
	if _, err := os.Stat(stray); err == nil {
		t.Fatal("a file no key points at survived replay")
	}
	if err := checkFiles(s); err != nil {
		t.Fatal(err)
	}
}

// Replay drops what expired while the process was down.
func TestReplayDropsExpired(t *testing.T) {
	lim := wbLimits(t.TempDir())
	now := time.Now()
	s, err := newStore(lim, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	sp := s.open("/x", lru(0))
	s.set(sp, "short", []byte("soon gone"), 50*time.Millisecond)
	s.set(sp, "long", []byte("stays"), time.Hour)
	s.close()
	for time.Since(now) < 60*time.Millisecond {
		time.Sleep(5 * time.Millisecond)
	}
	s2, err := newStore(lim, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s2.close)
	sp2 := s2.open("/x", lru(0))
	if has(s2, sp2, "short") || !has(s2, sp2, "long") {
		t.Fatal("replay should drop the expired key and keep the other")
	}
	if err := checkFiles(s2); err != nil {
		t.Fatal(err)
	}
}

// The index is rewritten once it holds far more records than keys.
func TestIndexCompacts(t *testing.T) {
	old := compactAt
	compactAt = 64
	t.Cleanup(func() { compactAt = old })
	dir := t.TempDir()
	s := newTestStore(t, wbLimits(dir))
	sp := s.open("/x", lru(0))
	for i := range 3 * compactAt {
		s.set(sp, fmt.Sprint(i%10), []byte(fmt.Sprint(i)), 0)
		if i%20 == 0 {
			s.flush()
		}
	}
	s.flush()
	n := lines(t, filepath.Join(s.lim.dir, indexFile))
	if n > compactAt+4*10+100 {
		t.Fatalf("index has %d lines for 10 keys: not compacted", n)
	}
	s = reopen(t, s, false)
	sp = s.open("/x", lru(0))
	for k := range 10 {
		if !has(s, sp, fmt.Sprint(k)) {
			t.Fatalf("key %d lost across compaction and restart", k)
		}
	}
	if got := val(s, sp, "9"); got != fmt.Sprint((3*compactAt-1)/10*10-1) {
		t.Fatalf("key 9 is %q after restart", got)
	}
	if n := lines(t, filepath.Join(s.lim.dir, indexFile)); n != 10 {
		t.Fatalf("index has %d lines after replay, want 10", n)
	}
}

func lines(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	for sc := bufio.NewScanner(f); sc.Scan(); {
		n++
	}
	return n
}

// A crash with a Set still in flight loses the new value but never brings back the one it replaced: the Set records
// the old value's removal in the index before it returns, and only the new value's file waits for the IO goroutine.
func TestCrashMidSetNeverStale(t *testing.T) {
	for _, item := range []bool{false, true} {
		t.Run(fmt.Sprintf("item=%v", item), func(t *testing.T) {
			dir := t.TempDir()
			s := newTestStore(t, wbLimits(dir))
			sp := s.open("/x", lru(0))
			put(s, sp, "k", "old")
			put(s, sp, "other", "untouched")
			s.flush()

			release := make(chan struct{})
			s.io.beforeOp = func() { <-release } // the new value's write never runs before the "crash"
			if item {
				s.setItem(sp, "k", &thing{s: "new"}, 0)
			} else {
				put(s, sp, "k", "new")
			}
			crashed := t.TempDir() // the disk as it is at this moment
			if err := copyDir(dir, crashed); err != nil {
				t.Fatal(err)
			}
			close(release)

			s2, err := newStore(wbLimits(crashed), slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(s2.close)
			sp2 := s2.open("/x", lru(0))
			if v, ok := s2.get(sp2, "k"); ok {
				t.Fatalf("after the crash k is %q: want a miss (the new value was in flight)", v)
			}
			if val(s2, sp2, "other") != "untouched" {
				t.Fatal("a key the crash didn't touch was lost")
			}
			if err := checkFiles(s2); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
}

// A Set that replaces a key on disk, a Delete and a Clear fsync the index before they return; a Set of a new key, or
// of a key only in memory, doesn't.
func TestIndexSyncs(t *testing.T) {
	s := newTestStore(t, wbLimits(t.TempDir()))
	sp := s.open("/x", lru(0))
	syncs := func() int { s.idx.mu.Lock(); defer s.idx.mu.Unlock(); return s.idx.syncs }
	put(s, sp, "a", "1")
	s.setItem(sp, "m", &live{}, 0)
	s.setItem(sp, "m", &live{}, 0) // memory only: no file to let go of
	if n := syncs(); n != 0 {
		t.Fatalf("%d syncs for new keys and memory-only values, want 0", n)
	}
	s.flush()
	put(s, sp, "a", "2") // replaces a value on disk
	if n := syncs(); n != 1 {
		t.Fatalf("%d syncs after replacing a key on disk, want 1", n)
	}
	s.flush()
	s.del(sp, "a")
	put(s, sp, "b", "x")
	s.flush()
	s.clear(sp)
	if n := syncs(); n != 3 {
		t.Fatalf("%d syncs after a Delete and a Clear, want 3", n)
	}
}
