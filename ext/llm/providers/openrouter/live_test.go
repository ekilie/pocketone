package openrouter

// Live tests against OpenRouter. P1_TEST_LIVE=1 runs the model list (no key); P1_TEST_OPENROUTER_KEY runs chat,
// stream, schema, a picture, speech and embeddings (a fraction of a cent each). Files go to $P1_TEST_OUT when set.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pocketone.dev/ext/llm/providers/oai"
)

func live(t *testing.T, needKey bool) *Client {
	t.Helper()
	key := os.Getenv("P1_TEST_OPENROUTER_KEY")
	if needKey && key == "" {
		t.Skip("set P1_TEST_OPENROUTER_KEY to run")
	}
	if !needKey && key == "" && os.Getenv("P1_TEST_LIVE") == "" {
		t.Skip("set P1_TEST_LIVE=1 or P1_TEST_OPENROUTER_KEY to run")
	}
	return New("", key)
}

func save(name string, f oai.File) {
	if dir := os.Getenv("P1_TEST_OUT"); dir != "" && len(f.Data) > 0 {
		_ = os.WriteFile(filepath.Join(dir, "openrouter-"+name), f.Data, 0o644)
	}
}

// cheapest picks the model making kind and served by endpoint with the lowest listed price among prefs, else the first.
func pick(t *testing.T, ms []Model, kind, endpoint string, prefs ...string) string {
	t.Helper()
	ok := func(m Model) bool { return m.Makes(kind) && (endpoint == "" || m.Serves(endpoint)) }
	for _, p := range prefs {
		for _, m := range ms {
			if m.ID == p && ok(m) {
				return p
			}
		}
	}
	for _, m := range ms {
		if ok(m) {
			return m.ID
		}
	}
	t.Skipf("no %s model on %s", kind, endpoint)
	return ""
}

func TestLiveModels(t *testing.T) {
	c := live(t, false)
	ms, err := c.Models(context.Background(), "all")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, m := range ms {
		for _, o := range m.Architecture.Outputs {
			kinds[o]++
		}
	}
	t.Logf("%d models; outputs %v", len(ms), kinds)
	if kinds["text"] == 0 {
		t.Fatal("no text models")
	}
}

func TestLiveAll(t *testing.T) {
	c := live(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	ms, err := c.Models(ctx, "all")
	if err != nil {
		t.Fatal(err)
	}
	text := "openai/gpt-4o-mini"
	msgs := func(s string) []oai.Message { return []oai.Message{{Role: "user", Content: s}} }

	t.Run("chat", func(t *testing.T) {
		r, err := c.Chat(ctx, oai.ChatRequest{Model: text, MaxTokens: 50, Messages: msgs("Say OK.")})
		if err != nil || !strings.Contains(strings.ToUpper(r.Text()), "OK") {
			t.Fatalf("%q %v", r.Text(), err)
		}
		cost := 0.0
		if r.Usage.Cost != nil {
			cost = *r.Usage.Cost
		}
		t.Logf("%s: %q cost $%.6f", text, r.Text(), cost)
	})
	t.Run("stream", func(t *testing.T) {
		r, err := c.ChatStream(ctx, oai.ChatRequest{Model: text, MaxTokens: 50, Messages: msgs("Count from 1 to 5.")}, nil)
		if err != nil || !strings.Contains(r.Text(), "5") {
			t.Fatalf("%q %v", r.Text(), err)
		}
	})
	t.Run("schema", func(t *testing.T) {
		r, err := c.Chat(ctx, oai.ChatRequest{Model: text, MaxTokens: 50, Messages: msgs("Label this ticket: I was charged twice."),
			ResponseFormat: &oai.ResponseFormat{Type: "json_schema", JSONSchema: &oai.JSONSchema{Name: "answer", Strict: true,
				Schema: json.RawMessage(`{"type":"object","properties":{"label":{"type":"string","enum":["bug","billing","question"]}},"required":["label"],"additionalProperties":false}`)}}})
		var v struct{ Label string }
		if err != nil || json.Unmarshal([]byte(r.Text()), &v) != nil || v.Label != "billing" {
			t.Fatalf("%q %v", r.Text(), err)
		}
	})
	t.Run("image", func(t *testing.T) {
		m := pick(t, ms, "image", "/api/v1/images", "black-forest-labs/flux-1-schnell", "black-forest-labs/flux.2-klein-4b")
		r, err := c.Images(ctx, oai.ImageRequest{Model: m, Prompt: "a small lighthouse at dusk, watercolor"})
		if err != nil || len(r.Images) == 0 {
			t.Fatalf("%s: %v %s", m, err, r.Raw)
		}
		t.Logf("%s: %d bytes %s usage %s", m, len(r.Images[0].Data), r.Images[0].ContentType, r.Usage)
		save("image.png", r.Images[0])
	})
	t.Run("speech", func(t *testing.T) {
		m := pick(t, ms, "speech", "/api/v1/audio/speech")
		var voice string
		for _, x := range ms {
			if x.ID == m && len(x.Voices) > 0 {
				voice = x.Voices[0]
			}
		}
		f, err := c.Speech(ctx, oai.SpeechRequest{Model: m, Input: "Once upon a time, a small lighthouse kept watch.", Voice: voice})
		if err != nil {
			t.Fatalf("%s: %v", m, err)
		}
		t.Logf("%s voice %q: %d bytes %s", m, voice, len(f.Data), f.ContentType)
		save("speech.mp3", f)
	})
	t.Run("embed", func(t *testing.T) {
		m := pick(t, ms, "embeddings", "/api/v1/embeddings", "openai/text-embedding-3-small")
		r, err := c.Embed(ctx, oai.EmbedRequest{Model: m, Input: []string{"bedtime story", "lighthouse"}})
		if err != nil || len(r.Vectors) != 2 || len(r.Vectors[0]) == 0 {
			t.Fatalf("%s: %v", m, err)
		}
		t.Logf("%s: 2 vectors of %d", m, len(r.Vectors[0]))
	})
}
