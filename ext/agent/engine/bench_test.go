package engine

import (
	"context"
	"os"
	"strconv"
	"testing"
)

// GGUF=... go test -run XXX -bench Decode -cpuprofile cpu.out
func BenchmarkDecode(b *testing.B) {
	p := os.Getenv("GGUF")
	if p == "" {
		b.Skip()
	}
	th, _ := strconv.Atoi(os.Getenv("THREADS"))
	if th == 0 {
		th = 8
	}
	m, err := Load(p, LoadOptions{Threads: th})
	if err != nil {
		b.Fatal(err)
	}
	defer m.Close()
	prompt := os.Getenv("PROMPT")
	if prompt == "" {
		prompt = "The old lighthouse keeper climbed the spiral stairs one last time"
	}
	gen, _ := strconv.Atoi(os.Getenv("GEN"))
	if gen == 0 {
		gen = 128
	}
	m.Generate(context.Background(), prompt, GenerateOptions{MaxTokens: 8, IgnoreEOS: true}, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, _ := m.Generate(context.Background(), prompt, GenerateOptions{MaxTokens: gen, IgnoreEOS: true}, nil)
		b.ReportMetric(float64(res.PromptTokens)/res.Prefill.Seconds(), "prefill-tok/s")
		b.ReportMetric(float64(res.OutputTokens-1)/res.Decode.Seconds(), "decode-tok/s")
	}
}
