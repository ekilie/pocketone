package cloudflare

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pocketone.dev/ext/agent/providers/oai"
)

func TestDriver(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nxxxx")
	var lastChat map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`)
			return
		}
		p := r.URL.Path
		switch {
		case p == "/ai/v1/chat/completions":
			_ = json.NewDecoder(r.Body).Decode(&lastChat)
			_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"{\"label\":\"billing\"}"},"finish_reason":"stop"}]}`)
		case p == "/ai/models/search":
			_, _ = io.WriteString(w, `{"success":true,"result":[{"id":"u1","name":"@cf/a/flux","task":{"name":"Text-to-Image"},
				"properties":[{"property_id":"price","value":[{"unit":"per 512 by 512 tile","price":0.00005,"currency":"USD"}]}]}]}`)
		case p == "/ai/run/@cf/a/flux":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in["prompt"] != "cat" || in["steps"] != float64(4) {
				w.WriteHeader(400)
				return
			}
			_, _ = io.WriteString(w, `{"success":true,"result":{"image":"`+base64.StdEncoding.EncodeToString(png)+`"}}`)
		case p == "/ai/run/@cf/b/flux-2-dev":
			if err := r.ParseMultipartForm(1 << 20); err != nil || r.FormValue("prompt") != "cat" {
				w.WriteHeader(400)
				return
			}
			_, _ = io.WriteString(w, `{"result":{"image":"`+base64.StdEncoding.EncodeToString(png)+`"},"success":true}`)
		case p == "/ai/run/@cf/deepgram/aura-2-en":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = io.WriteString(w, "ID3speech")
		case p == "/ai/run/@cf/openai/whisper":
			b, _ := io.ReadAll(r.Body)
			_, _ = io.WriteString(w, `{"success":true,"result":{"text":"heard `+string(b)+`"}}`)
		case p == "/ai/run/@cf/openai/whisper-large-v3-turbo":
			var in struct{ Audio string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			b, _ := base64.StdEncoding.DecodeString(in.Audio)
			_, _ = io.WriteString(w, `{"success":true,"result":{"text":"turbo `+string(b)+`"}}`)
		default:
			w.WriteHeader(400)
			_, _ = io.WriteString(w, `{"success":false,"errors":[{"code":5007,"message":"No such model"}]}`)
		}
	}))
	defer srv.Close()
	c := NewAt(srv.URL+"/ai", "tok")
	ctx := context.Background()

	r, err := c.Chat(ctx, oai.ChatRequest{Model: "@cf/m", ResponseFormat: &oai.ResponseFormat{Type: "json_schema",
		JSONSchema: &oai.JSONSchema{Name: "answer", Schema: json.RawMessage(`{"type":"object"}`)}}})
	if err != nil || r.Text() != `{"label":"billing"}` {
		t.Fatalf("%+v %v", r, err)
	}
	if rf, _ := lastChat["response_format"].(map[string]any); rf["json_schema"].(map[string]any)["type"] != "object" {
		t.Fatalf("schema not sent Cloudflare's way: %v", lastChat["response_format"])
	}

	ms, err := c.Models(ctx)
	if err != nil || len(ms) != 1 || ms[0].Kind() != "image" || ms[0].Prices()[0].Unit != "per 512 by 512 tile" {
		t.Fatalf("%+v %v", ms, err)
	}
	for _, m := range []string{"@cf/a/flux", "@cf/b/flux-2-dev"} {
		f, err := c.Image(ctx, ImageRequest{Model: m, Prompt: "cat", Steps: 4})
		if err != nil || f.ContentType != "image/png" {
			t.Fatalf("%s: %+v %v", m, f, err)
		}
	}
	sp, err := c.Speech(ctx, SpeechRequest{Model: "@cf/deepgram/aura-2-en", Text: "hi", Voice: "luna"})
	if err != nil || string(sp.Data) != "ID3speech" || sp.ContentType != "audio/mpeg" {
		t.Fatalf("%+v %v", sp, err)
	}
	for m, want := range map[string]string{"@cf/openai/whisper": "heard words", "@cf/openai/whisper-large-v3-turbo": "turbo words"} {
		tr, err := c.Transcribe(ctx, m, []byte("words"), "")
		if err != nil || tr.Text != want {
			t.Fatalf("%s: %+v %v", m, tr, err)
		}
	}
	var e *oai.Error
	if _, err := c.Run(ctx, "@cf/nope", map[string]any{}); !errors.As(err, &e) || e.Code != "5007" || !strings.Contains(e.Message, "No such model") {
		t.Fatalf("%#v", err)
	}
	if _, err := NewAt(srv.URL+"/ai", "bad").Models(ctx); !errors.As(err, &e) || e.Status != 401 || e.Code != "10000" {
		t.Fatalf("%#v", err)
	}
}
