package engine

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
)

// GGUF=<file> [GEN=32] go test -run TestGreedyTokens -v
// Prints the greedy continuation's token ids, for comparing builds and kernels.
func TestGreedyTokens(t *testing.T) {
	p := os.Getenv("GGUF")
	if p == "" {
		t.Skip("GGUF not set")
	}
	gen, _ := strconv.Atoi(os.Getenv("GEN"))
	if gen == 0 {
		gen = 32
	}
	m, err := Load(p, LoadOptions{Threads: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	prompts := []string{
		"The old lighthouse keeper climbed the spiral stairs one last time, carrying a lantern and a notebook full of stories he had never told anyone. He began to",
		ChatPrompt([]Message{{Role: "user", Content: "Name three primary colors, comma separated."}}),
	}
	for i, pr := range prompts {
		res, err := m.Generate(context.Background(), pr, GenerateOptions{MaxTokens: gen, IgnoreEOS: true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("kernels=%s prompt%d tokens=%v\n", kernelName, i, res.Tokens)
		fmt.Printf("kernels=%s prompt%d text=%q\n", kernelName, i, res.Text)
	}
}

// GGUF=<file> go test -run TestLogitsAtDivergence -v
// Prints the top logits where NEON and AVX2 greedy runs part ways on prompt 0.
func TestLogitsAtDivergence(t *testing.T) {
	p := os.Getenv("GGUF")
	if p == "" {
		t.Skip("GGUF not set")
	}
	m, err := Load(p, LoadOptions{Threads: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ids := m.Tokenize("The old lighthouse keeper climbed the spiral stairs one last time, carrying a lantern and a notebook full of stories he had never told anyone. He began to")
	ids = append(ids, 25367, 279, 31149, 11, 323, 279, 1156, 3019, 572, 264)
	s := m.newState(len(ids)+1, len(ids))
	m.forward(s, ids, 0)
	fmt.Printf("kernels=%s top=%s\n", kernelName, top(s.logits, 4))
}
