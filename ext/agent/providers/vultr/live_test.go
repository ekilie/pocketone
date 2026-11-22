package vultr

// Live tests against Vultr's real API. They run only when asked:
//
//	P1_TEST_LIVE=1                 the model list (needs no key)
//	P1_TEST_VULTR_KEY=<key>        chat, stream, schema, image, speech, transcription, embeddings, a 10 s music job
//	P1_TEST_VULTR_VIDEO=1          also a video job (slow; costs more)
//
// Each call costs a fraction of a cent except video. Generated files are written to $P1_TEST_OUT when it's set.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pocketone.dev/ext/agent/providers/oai"
)

func live(t *testing.T, needKey bool) *Client {
	t.Helper()
	key := os.Getenv("P1_TEST_VULTR_KEY")
	if needKey && key == "" {
		t.Skip("set P1_TEST_VULTR_KEY to run")
	}
	if !needKey && key == "" && os.Getenv("P1_TEST_LIVE") == "" {
		t.Skip("set P1_TEST_LIVE=1 or P1_TEST_VULTR_KEY to run")
	}
	return New(os.Getenv("P1_TEST_VULTR_URL"), key)
}

func save(t *testing.T, name string, f oai.File) {
	if dir := os.Getenv("P1_TEST_OUT"); dir != "" && len(f.Data) > 0 {
		_ = os.WriteFile(filepath.Join(dir, "vultr-"+name), f.Data, 0o644)
	}
}

// pick returns the first model in prefs that the list has making kind, else the first model making kind.
func pick(t *testing.T, ms []Model, kind string, prefs ...string) string {
	t.Helper()
	have := map[string]Model{}
	for _, m := range ms {
		have[m.ID] = m
	}
	for _, p := range prefs {
		if m, ok := have[p]; ok && m.Makes(kind) {
			return p
		}
	}
	for _, m := range ms {
		if m.Makes(kind) {
			return m.ID
		}
	}
	t.Skipf("no %s model", kind)
	return ""
}

func TestLiveModels(t *testing.T) {
	c := live(t, false)
	ms, err := c.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, m := range ms {
		for _, o := range m.Outputs {
			kinds[o.Type]++
		}
	}
	t.Logf("%d models; outputs %v", len(ms), kinds)
	if len(ms) == 0 || kinds["text"] == 0 {
		t.Fatal("no text models")
	}
}

func TestLiveAll(t *testing.T) {
	c := live(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ms, err := c.Models(ctx)
	if err != nil {
		t.Fatal(err)
	}
	text := pick(t, ms, "text", "qwen3.8-flash-next", "glm-5.3-flash", "glm-5.2")

	t.Run("chat", func(t *testing.T) {
		r, err := c.Chat(ctx, oai.ChatRequest{Model: text, MaxTokens: 1000, Messages: []oai.Message{{Role: "user", Content: "Say OK."}}})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %q (reasoning %d chars) usage %+v", text, r.Text(), len(r.Choices[0].Message.Reasoning), r.Usage)
		if !strings.Contains(strings.ToUpper(r.Text()), "OK") {
			t.Fatalf("answer %q", r.Text())
		}
	})
	t.Run("stream", func(t *testing.T) {
		n := 0
		r, err := c.ChatStream(ctx, oai.ChatRequest{Model: text, MaxTokens: 1000, Messages: []oai.Message{{Role: "user", Content: "Count from 1 to 5."}}},
			func(d oai.Delta) bool { n++; return true })
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%d pieces: %q usage %+v", n, r.Text(), r.Usage)
		if !strings.Contains(r.Text(), "5") {
			t.Fatalf("answer %q", r.Text())
		}
	})
	t.Run("schema", func(t *testing.T) {
		r, err := c.Chat(ctx, oai.ChatRequest{Model: text, MaxTokens: 1000,
			Messages: []oai.Message{{Role: "system", Content: "Label the support ticket."}, {Role: "user", Content: "I was charged twice."}},
			ResponseFormat: &oai.ResponseFormat{Type: "json_schema", JSONSchema: &oai.JSONSchema{Name: "answer",
				Schema: json.RawMessage(`{"type":"object","properties":{"label":{"enum":["bug","billing","question"]}},"required":["label"]}`)}}})
		if err != nil {
			t.Fatal(err)
		}
		var v struct{ Label string }
		if err := json.Unmarshal([]byte(r.Text()), &v); err != nil || v.Label != "billing" {
			t.Fatalf("answer %q: %v", r.Text(), err)
		}
	})
	t.Run("image", func(t *testing.T) {
		m := pick(t, ms, "image", "z-image-turbo")
		r, err := c.Images(ctx, oai.ImageRequest{Model: m, Prompt: "a small lighthouse at dusk, watercolor", Size: "512x512", ResponseFormat: "b64_json"})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Images) == 0 || (len(r.Images[0].Data) == 0 && r.Images[0].URL == "") {
			t.Fatalf("no image: %s", r.Raw)
		}
		t.Logf("%s: %d bytes %s url=%q usage %s", m, len(r.Images[0].Data), r.Images[0].ContentType, r.Images[0].URL, r.Usage)
		save(t, "image.png", r.Images[0])
	})
	var speech oai.File
	t.Run("speech", func(t *testing.T) {
		m := pick(t, ms, "speech", "qwen3-tts-12hz-1.7b", "chatterbox")
		vs, err := c.Voices(ctx, m)
		if err != nil {
			t.Logf("voices: %v", err)
		}
		req := oai.SpeechRequest{Model: m, Input: "Once upon a time, a small lighthouse kept watch over the sea.", ResponseFormat: "mp3"}
		if len(vs) > 0 {
			req.Voice = vs[0].ID
		}
		f, err := c.Speech(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s (%d voices): %d bytes %s", m, len(vs), len(f.Data), f.ContentType)
		speech = f
		save(t, "speech.mp3", f)
	})
	t.Run("transcribe", func(t *testing.T) {
		if len(speech.Data) == 0 {
			t.Skip("no speech to transcribe")
		}
		m := pick(t, ms, "transcription", "whisper-large-v3-turbo")
		r, err := c.Transcribe(ctx, oai.TranscribeRequest{Model: m, Audio: speech.Data, Filename: "speech.mp3"})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %q", m, r.Text)
		if !strings.Contains(strings.ToLower(r.Text), "lighthouse") {
			t.Fatalf("transcript %q", r.Text)
		}
	})
	t.Run("embed", func(t *testing.T) {
		m := pick(t, ms, "embeddings", "qwen3-embedding-4b")
		r, err := c.Embed(ctx, oai.EmbedRequest{Model: m, Input: []string{"bedtime story", "lighthouse"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Vectors) != 2 || len(r.Vectors[0]) == 0 {
			t.Fatalf("vectors %d", len(r.Vectors))
		}
		t.Logf("%s: 2 vectors of %d", m, len(r.Vectors[0]))
	})
	t.Run("music", func(t *testing.T) {
		m := pick(t, ms, "audio", "ace-step-1.5")
		j, err := c.Music(ctx, MusicRequest{Model: m, Prompt: "gentle lullaby, music box, slow", Instrumental: true, Duration: 10, ResponseFormat: "mp3"})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("job %s %s", j.ID, j.Status)
		if j, err = c.Wait(ctx, "music", j, 2*time.Second); err != nil {
			t.Fatal(err)
		}
		f, err := c.Content(ctx, "music", j.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d bytes %s usage %s", m, len(f.Data), f.ContentType, j.Usage)
		save(t, "music.mp3", f)
	})
	t.Run("video", func(t *testing.T) {
		if os.Getenv("P1_TEST_VULTR_VIDEO") == "" {
			t.Skip("set P1_TEST_VULTR_VIDEO=1 to run")
		}
		m := pick(t, ms, "video")
		j, err := c.Video(ctx, VideoRequest{Model: m, Prompt: "a lighthouse beam sweeping over calm sea at night", Duration: 5})
		if err != nil {
			t.Fatal(err)
		}
		if j, err = c.Wait(ctx, "video", j, 5*time.Second); err != nil {
			t.Fatal(err)
		}
		f, err := c.Content(ctx, "video", j.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d bytes %s usage %s", m, len(f.Data), f.ContentType, j.Usage)
		save(t, "video.mp4", f)
	})
}
