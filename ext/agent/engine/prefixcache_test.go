package engine

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"
)

// A prompt that starts like an earlier one reuses the earlier one's keys and values: the logits it ends with match a
// full read (within float rounding, since the prompt splits into different prefill chunks), Generate reports the
// cached tokens, and the cache stays inside its budget. GGUF=<a Qwen3 .gguf> go test -run TestPromptCache -v
func TestPromptCache(t *testing.T) {
	path := os.Getenv("GGUF")
	if path == "" {
		t.Skip("set GGUF to a Qwen3 GGUF file")
	}
	const budget = 64 << 20
	m, err := Load(path, LoadOptions{Threads: 4, PromptCache: budget})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	preamble := "You are the help assistant for Acme Notes, a note-taking app. Answer from the excerpts below. " +
		strings.Repeat("Notes sync across devices within a few seconds of saving. ", 12)
	a := m.ChatPrompt([]Message{{Role: "system", Content: preamble}, {Role: "user", Content: "How fast do notes sync?"}})
	b := m.ChatPrompt([]Message{{Role: "system", Content: preamble}, {Role: "user", Content: "Can I share a notebook?"}})

	ra, err := m.Generate(context.Background(), a, GenerateOptions{MaxTokens: 4}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ra.CachedTokens != 0 {
		t.Fatalf("first prompt: %d cached tokens, want 0", ra.CachedTokens)
	}
	rb, err := m.Generate(context.Background(), b, GenerateOptions{MaxTokens: 4}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := m.Tokenize(b)
	if rb.CachedTokens < prefixMin || rb.CachedTokens >= len(ids) {
		t.Fatalf("second prompt: %d of %d tokens cached", rb.CachedTokens, len(ids))
	}
	t.Logf("prompt b: %d tokens, %d from the cache; prefill %v (prompt a: %v)", len(ids), rb.CachedTokens, rb.Prefill, ra.Prefill)

	// the logits after b: read fully, and resumed after the cached prefix
	full := m.newState(len(ids)+8, prefillBatch)
	m.forward(full, ids, 0)
	e, n := m.lookupPrefix(ids)
	if e == nil {
		t.Fatal("no cached prefix for b")
	}
	resumed := m.newState(len(ids)+8, prefillBatch)
	m.restorePrefix(resumed, e, n)
	m.forward(resumed, ids[n:], n)
	// the same work without the cache: read a (which filled the cache) whole, then b's tail after the shared part. The
	// cache must reproduce it exactly. (Reading only b[:n] first would batch those tokens differently from how a's read
	// computed them, which moves the result by rounding, not by anything the cache did.)
	aids := m.Tokenize(a)
	split := m.newState(max(len(ids), len(aids))+8, prefillBatch)
	m.forward(split, aids, 0)
	m.forward(split, ids[n:], n)
	diff := func(x, y []float32) (worst float64) {
		for i := range x {
			worst = math.Max(worst, math.Abs(float64(x[i]-y[i])))
		}
		return worst
	}
	if d := diff(split.logits, resumed.logits); d != 0 {
		t.Fatalf("logits from the cache differ from the same split read without it by %g", d)
	}
	// against one read of the whole prompt, splitting changes how the prompt is batched, which moves logits a little
	t.Logf("cached vs uncached: identical; vs one read of b: largest logit difference %.3f", diff(full.logits, split.logits))
	if argmax(full.logits) != argmax(resumed.logits) {
		t.Logf("note: the top token differs between one read and the split (a near tie)")
	}

	st := m.PromptCache()
	if st.Bytes > budget || st.Entries == 0 || st.Hits == 0 {
		t.Fatalf("cache stats: %+v (budget %d)", st, budget)
	}
}

func argmax(x []float32) int {
	best := 0
	for i, v := range x {
		if v > x[best] {
			best = i
		}
	}
	return best
}
