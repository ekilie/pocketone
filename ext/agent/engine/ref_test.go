package engine

import (
	"fmt"
	"os"
	"sort"
	"testing"
)

func top(l []float32, k int) string {
	idx := make([]int, len(l))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return l[idx[a]] > l[idx[b]] })
	s := ""
	for _, i := range idx[:k] {
		s += fmt.Sprintf("%d:%.3f ", i, l[i])
	}
	return s
}

func TestFloatRef(t *testing.T) {
	p := os.Getenv("GGUF")
	if p == "" {
		t.Skip()
	}
	m, err := Load(p, LoadOptions{Threads: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	prompt := os.Getenv("PROMPT")
	ids := m.Tokenize(prompt)
	for _, fr := range []bool{false, true} {
		floatRef = fr
		s := m.newState(len(ids)+1, min(len(ids), prefillBatch))
		for pos := 0; pos < len(ids); pos += prefillBatch {
			m.forward(s, ids[pos:min(pos+prefillBatch, len(ids))], pos)
		}
		fmt.Println("floatRef", fr, top(s.logits, 5))
		// also: token by token (no batching)
		s2 := m.newState(len(ids)+1, 1)
		for pos := range ids {
			m.forward(s2, ids[pos:pos+1], pos)
		}
		fmt.Println("  one-by-one", top(s2.logits, 5))
	}
	floatRef = false
}
