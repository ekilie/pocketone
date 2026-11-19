package engine

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

func TestConcurrentGenerate(t *testing.T) {
	p := os.Getenv("GGUF")
	if p == "" {
		t.Skip("GGUF not set")
	}
	m, err := Load(p, LoadOptions{Threads: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	prompt := ChatPrompt([]Message{{Role: "user", Content: "Count from one to ten."}})
	want, err := m.Generate(context.Background(), prompt, GenerateOptions{MaxTokens: 16}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := m.Generate(context.Background(), prompt, GenerateOptions{MaxTokens: 16}, nil)
			if err != nil || got.Text != want.Text {
				t.Errorf("concurrent result differs: %q vs %q (%v)", got.Text, want.Text, err)
			}
		}()
	}
	wg.Wait()
	// cancellation stops promptly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	res, _ := m.Generate(ctx, prompt, GenerateOptions{MaxTokens: 2000, IgnoreEOS: true}, nil)
	if d := time.Since(start); d > time.Second || res.StopReason != "canceled" {
		t.Errorf("cancel took %v, reason %q", d, res.StopReason)
	}
	// callback stop and stop strings
	n := 0
	res, _ = m.Generate(context.Background(), prompt, GenerateOptions{MaxTokens: 50}, func(string) bool { n++; return n < 3 })
	if res.StopReason != "callback" {
		t.Errorf("callback stop: %q", res.StopReason)
	}
	res, _ = m.Generate(context.Background(), prompt, GenerateOptions{MaxTokens: 50, Stop: []string{"three", "Three", "3"}}, nil)
	if res.StopReason != "stop" {
		t.Errorf("stop string: %q %q", res.StopReason, res.Text)
	}
	so := GenerateOptions{MaxTokens: 24, Temperature: 0.8, TopP: 0.95, TopK: 40, Seed: 42}
	a, _ := m.Generate(context.Background(), prompt, so, nil)
	b, _ := m.Generate(context.Background(), prompt, so, nil)
	so.TopK = 0
	c, _ := m.Generate(context.Background(), prompt, so, nil)
	if a.Text != b.Text {
		t.Errorf("same seed, different text: %q vs %q", a.Text, b.Text)
	}
	t.Logf("%q | sampled %q | topk0 %q", want.Text, a.Text, c.Text)
}
