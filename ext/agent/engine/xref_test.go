package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

// Cross-kernel check: one build writes greedy continuations with its own
// log-probabilities, another build (other kernels or architecture)
// teacher-forces them and reports top-1 agreement and log-prob drift.
//
//	GGUF=f XREF_PROMPTS=prompts.txt XREF_OUT=ref.json go test -run TestXrefWrite
//	GGUF=f XREF_IN=ref.json go test -run TestXrefCheck -v
//
// prompts.txt holds raw prompts separated by lines containing only "----".

type xrefCase struct {
	Prompt  []int
	Cont    []int
	Logprob []float64
	Kernels string
}

func TestXrefWrite(t *testing.T) {
	p, pf, out := os.Getenv("GGUF"), os.Getenv("XREF_PROMPTS"), os.Getenv("XREF_OUT")
	if p == "" || pf == "" || out == "" {
		t.Skip("GGUF, XREF_PROMPTS, XREF_OUT not set")
	}
	m, err := Load(p, LoadOptions{Threads: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	b, err := os.ReadFile(pf)
	if err != nil {
		t.Fatal(err)
	}
	var cases []xrefCase
	for _, pr := range strings.Split(string(b), "\n----\n") {
		ids := m.Tokenize(pr)
		res, err := m.GenerateTokens(context.Background(), ids, GenerateOptions{MaxTokens: 64, IgnoreEOS: true}, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, lp := m.teacherForce(ids, res.Tokens)
		cases = append(cases, xrefCase{ids, res.Tokens, lp, kernelName})
	}
	j, _ := json.Marshal(cases)
	if err := os.WriteFile(out, j, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestXrefCheck(t *testing.T) {
	p, in := os.Getenv("GGUF"), os.Getenv("XREF_IN")
	if p == "" || in == "" {
		t.Skip("GGUF, XREF_IN not set")
	}
	m, err := Load(p, LoadOptions{Threads: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	b, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	var cases []xrefCase
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		am, lp := m.teacherForce(c.Prompt, c.Cont)
		agree, first := 0, -1
		var sum, mx float64
		for k := range c.Cont {
			if am[k] == c.Cont[k] {
				agree++
			} else if first < 0 {
				first = k
			}
			d := math.Abs(lp[k] - c.Logprob[k])
			sum += d
			mx = math.Max(mx, d)
		}
		fmt.Printf("xref case%d (%d prompt tokens): %s vs %s: top-1 agreement %d/%d (first miss at %d), logprob |diff| mean %.3f max %.3f\n",
			i, len(c.Prompt), kernelName, c.Kernels, agree, len(c.Cont), first, sum/float64(len(c.Cont)), mx)
	}
}
