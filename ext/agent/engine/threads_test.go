package engine

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestF16RoundTrip(t *testing.T) {
	for _, f := range []float32{0, 1, -1, 0.5, 3.14159, -2.71828, 65504, 1e-5, -1e-7, 123.456} {
		got := f16Table[f32ToF16(f)]
		if diff := math.Abs(float64(got - f)); diff > math.Abs(float64(f))*1e-3+1e-7 {
			t.Fatalf("%v -> %v", f, got)
		}
	}
}

func loadTestModel(t *testing.T, o LoadOptions) *Model {
	t.Helper()
	path := os.Getenv("GGUF")
	if path == "" {
		t.Skip("set GGUF to a Qwen3 GGUF file")
	}
	o.Threads = 4
	m, err := Load(path, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

// A conversation on a thread reads only what's new: the second turn's prompt (the first turn, the model's answer, a new
// question) continues from the first call's state, answer included. With memory off, the state comes back from disk.
// GGUF=<a Qwen3 .gguf> go test -run TestThreads -v
func TestThreads(t *testing.T) {
	for _, tc := range []struct {
		name   string
		memory int64
	}{{"memory", 256 << 20}, {"disk", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			m := loadTestModel(t, LoadOptions{ThreadMemory: tc.memory})
			dir := t.TempDir()
			o := GenerateOptions{MaxTokens: 24, Thread: "chat1", ThreadDir: dir, ThreadDisk: 1 << 30}
			sys := Message{Role: "system", Content: "You are a terse assistant. " + strings.Repeat("Answer in one short sentence. ", 4)}
			q1 := Message{Role: "user", Content: "Name a color of the sky."}
			r1, err := m.Generate(context.Background(), m.ChatPrompt([]Message{sys, q1}), o, nil)
			if err != nil {
				t.Fatal(err)
			}
			waitDisk(m)
			if tc.memory == 0 {
				if n := len(m.Threads("", dir)); n != 1 {
					t.Fatalf("threads on disk: %d", n)
				}
			}
			p2 := m.ChatPrompt([]Message{sys, q1, {Role: "assistant", Content: r1.Text}, {Role: "user", Content: "And of grass?"}})
			r2, err := m.Generate(context.Background(), p2, o, nil)
			if err != nil {
				t.Fatal(err)
			}
			// everything up to the end of the first answer comes from the thread
			first := r1.PromptTokens + r1.OutputTokens - 2
			if r2.CachedTokens < first {
				t.Fatalf("second turn: %d cached of %d prompt tokens, want at least %d (the first turn and its answer)", r2.CachedTokens, r2.PromptTokens, first)
			}
			waitDisk(m)
			list := m.Threads("", dir)
			if len(list) != 1 || list[0].ID != "chat1" || list[0].Tokens < r2.PromptTokens {
				t.Fatalf("threads: %+v", list)
			}
			if other := m.Threads("someone-else", dir); len(other) != 0 {
				t.Fatalf("another scope sees the thread: %+v", other)
			}
			waitDisk(m)
			m.ForgetThread("", "chat1", dir)
			if list := m.Threads("", dir); len(list) != 0 {
				t.Fatalf("after forget: %+v", list)
			}
		})
	}
}

func waitDisk(m *Model) { m.FlushThreads() }

func timeAt(sec float64) time.Time { return time.Unix(1_700_000_000+int64(sec), 0) }

// Disk budgets evict the least recently used thread files.
func TestEvictDir(t *testing.T) {
	dir := t.TempDir()
	for i, name := range []string{"a.kv", "b.kv", "c.kv"} {
		p := filepath.Join(dir, "m", name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, make([]byte, 100), 0o644)
		mt := float64(i)
		_ = os.Chtimes(p, timeAt(mt), timeAt(mt))
	}
	evictDir(dir, 250)
	if _, err := os.Stat(filepath.Join(dir, "m", "a.kv")); !os.IsNotExist(err) {
		t.Fatal("the oldest file survived")
	}
	if _, err := os.Stat(filepath.Join(dir, "m", "c.kv")); err != nil {
		t.Fatal("the newest file was evicted")
	}
}

// Warm reads a prompt's start into the cache and pins it: a later call starting the same way reuses it, and other
// prompts don't push it out while it's pinned.
func TestWarm(t *testing.T) {
	m := loadTestModel(t, LoadOptions{PromptCache: 64 << 20})
	pre := m.ChatPrompt([]Message{{Role: "system", Content: "You help with Acme Notes. " + strings.Repeat("Notes sync in seconds. ", 10)}})
	pre = strings.TrimSuffix(pre, "<|im_start|>assistant\n<think>\n\n</think>\n\n")
	w, err := m.Warm(context.Background(), pre, "", "pin1")
	if err != nil || w.Kept == 0 || w.Kept != w.Tokens {
		t.Fatalf("warm: %+v %v", w, err)
	}
	if st := m.PromptCache(); st.Pinned == 0 {
		t.Fatalf("nothing pinned: %+v", st)
	}
	// fill the cache with other prompts; the pinned one stays
	for i := 0; i < 4; i++ {
		other := m.ChatPrompt([]Message{{Role: "system", Content: strings.Repeat("Unrelated filler text number "+string(rune('a'+i))+". ", 40)}, {Role: "user", Content: "hi"}})
		if _, err := m.Generate(context.Background(), other, GenerateOptions{MaxTokens: 1}, nil); err != nil {
			t.Fatal(err)
		}
	}
	r, err := m.Generate(context.Background(), pre+"<|im_start|>user\nHow fast?<|im_end|>\n<|im_start|>assistant\n", GenerateOptions{MaxTokens: 2}, nil)
	if err != nil || r.CachedTokens < w.Kept-1 {
		t.Fatalf("after warm: %d cached, want about %d (%v)", r.CachedTokens, w.Kept, err)
	}
	m.Unpin("pin1")
	if st := m.PromptCache(); st.Pinned != 0 {
		t.Fatalf("still pinned: %+v", st)
	}
}
