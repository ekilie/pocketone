package engine

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestGenerateSmoke(t *testing.T) {
	p := os.Getenv("GGUF")
	if p == "" {
		t.Skip("GGUF not set")
	}
	m, err := Load(p, LoadOptions{Threads: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	fmt.Printf("%+v\n", m.Info())
	prompt := ChatPrompt([]Message{{"user", "What is the capital of France? Answer in one sentence."}})
	ids := m.Tokenize(prompt)
	fmt.Println(len(ids), ids)
	res, err := m.Generate(context.Background(), prompt, GenerateOptions{MaxTokens: 40}, func(s string) bool { fmt.Print(s); return true })
	fmt.Println()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%d prompt, %d out, prefill %v, decode %v (%.1f tok/s), %s\n", res.PromptTokens, res.OutputTokens, res.Prefill, res.Decode,
		float64(res.OutputTokens-1)/res.Decode.Seconds(), res.StopReason)
}
