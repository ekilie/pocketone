package cloudflare

// Live tests against Workers AI with P1_TEST_CF_ACCOUNT and P1_TEST_CF_TOKEN (a token with Workers AI permission):
// the model list, chat, stream, schema, a picture, speech, transcription and embeddings. Workers AI's free daily
// allocation covers them. Files go to $P1_TEST_OUT when set.

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

func TestLive(t *testing.T) {
	acct, tok := os.Getenv("P1_TEST_CF_ACCOUNT"), os.Getenv("P1_TEST_CF_TOKEN")
	if acct == "" || tok == "" {
		t.Skip("set P1_TEST_CF_ACCOUNT and P1_TEST_CF_TOKEN to run")
	}
	c := New(acct, tok)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	save := func(name string, f oai.File) {
		if dir := os.Getenv("P1_TEST_OUT"); dir != "" && len(f.Data) > 0 {
			_ = os.WriteFile(filepath.Join(dir, "cloudflare-"+name), f.Data, 0o644)
		}
	}
	ms, err := c.Models(ctx)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string][]string{}
	for _, m := range ms {
		kinds[m.Kind()] = append(kinds[m.Kind()], m.ID)
	}
	for k, ids := range kinds {
		t.Logf("%s (%d): %s", k, len(ids), strings.Join(ids, " "))
	}
	text := "@cf/meta/llama-3.3-70b-instruct-fp8-fast"
	msgs := func(s string) []oai.Message { return []oai.Message{{Role: "user", Content: s}} }

	t.Run("chat", func(t *testing.T) {
		r, err := c.Chat(ctx, oai.ChatRequest{Model: text, MaxTokens: 50, Messages: msgs("Say OK.")})
		if err != nil || !strings.Contains(strings.ToUpper(r.Text()), "OK") {
			t.Fatalf("%q %v", r.Text(), err)
		}
	})
	t.Run("stream", func(t *testing.T) {
		r, err := c.ChatStream(ctx, oai.ChatRequest{Model: text, MaxTokens: 50, Messages: msgs("Count from 1 to 5.")}, nil)
		if err != nil || !strings.Contains(r.Text(), "5") {
			t.Fatalf("%q %v", r.Text(), err)
		}
	})
	t.Run("schema", func(t *testing.T) {
		r, err := c.Chat(ctx, oai.ChatRequest{Model: text, MaxTokens: 100, Messages: msgs("Label this ticket: I was charged twice."),
			ResponseFormat: &oai.ResponseFormat{Type: "json_schema", JSONSchema: &oai.JSONSchema{Name: "answer",
				Schema: json.RawMessage(`{"type":"object","properties":{"label":{"type":"string","enum":["bug","billing","question"]}},"required":["label"]}`)}}})
		var v struct{ Label string }
		if err != nil || json.Unmarshal([]byte(r.Text()), &v) != nil || v.Label != "billing" {
			t.Fatalf("%q %v", r.Text(), err)
		}
	})
	t.Run("image", func(t *testing.T) {
		f, err := c.Image(ctx, ImageRequest{Model: "@cf/black-forest-labs/flux-1-schnell", Prompt: "a small lighthouse at dusk, watercolor", Steps: 4})
		if err != nil || len(f.Data) == 0 {
			t.Fatalf("%+v %v", f.ContentType, err)
		}
		t.Logf("flux-1-schnell: %d bytes %s", len(f.Data), f.ContentType)
		save("image.jpg", f)
	})
	var speech oai.File
	t.Run("speech", func(t *testing.T) {
		f, err := c.Speech(ctx, SpeechRequest{Model: "@cf/deepgram/aura-2-en", Text: "Once upon a time, a small lighthouse kept watch over the sea.", Voice: "luna"})
		if err != nil || len(f.Data) == 0 {
			t.Fatal(err)
		}
		t.Logf("aura-2-en: %d bytes %s", len(f.Data), f.ContentType)
		speech = f
		save("speech.mp3", f)
	})
	t.Run("transcribe", func(t *testing.T) {
		if len(speech.Data) == 0 {
			t.Skip("no speech")
		}
		for _, m := range []string{"@cf/openai/whisper-large-v3-turbo", "@cf/openai/whisper"} {
			r, err := c.Transcribe(ctx, m, speech.Data, "")
			if err != nil {
				t.Errorf("%s: %v", m, err)
				continue
			}
			t.Logf("%s: %q", m, r.Text)
			if !strings.Contains(strings.ToLower(r.Text), "lighthouse") {
				t.Errorf("%s transcript %q", m, r.Text)
			}
		}
	})
	t.Run("embed", func(t *testing.T) {
		r, err := c.Embed(ctx, oai.EmbedRequest{Model: "@cf/baai/bge-base-en-v1.5", Input: []string{"bedtime story", "lighthouse"}})
		if err != nil || len(r.Vectors) != 2 || len(r.Vectors[0]) == 0 {
			t.Fatalf("%v", err)
		}
		t.Logf("bge-base: 2 vectors of %d", len(r.Vectors[0]))
	})
}
