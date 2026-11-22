package oai

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
)

// fake answers the OpenAI-shaped paths the client calls, recording the last JSON body.
func fake(t *testing.T) (*Client, *map[string]any) {
	t.Helper()
	last := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"error":{"message":"bad key","type":"auth"}}`)
			return
		}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			last = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&last)
		}
		switch r.URL.Path {
		case "/v1/chat/completions":
			if last["stream"] == true {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, l := range []string{
					": keepalive",
					`data: {"id":"c1","model":"m","choices":[{"delta":{"reasoning_content":"hmm"}}]}`,
					`data: {"choices":[{"delta":{"content":"Hel"}}]}`,
					`data: {"choices":[{"delta":{"content":"lo"},"finish_reason":"stop"}]}`,
					`data: {"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
					"data: [DONE]",
				} {
					_, _ = io.WriteString(w, l+"\n\n")
				}
				return
			}
			_, _ = io.WriteString(w, `{"id":"c1","model":"m","choices":[{"message":{"role":"assistant","content":"Hi","reasoning_content":"think"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4,"cost":0.001}}`)
		case "/v1/images/generations":
			png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nxxxx"))
			_, _ = io.WriteString(w, `{"created":1,"data":[{"b64_json":"`+png+`"},{"url":"https://x/y.png"}],"usage":{"cost":0.02}}`)
		case "/v1/audio/speech":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("ID3audio"))
		case "/v1/audio/transcriptions":
			_ = r.ParseMultipartForm(1 << 20)
			f, _, err := r.FormFile("file")
			if err != nil || r.FormValue("model") != "w" {
				w.WriteHeader(400)
				return
			}
			b, _ := io.ReadAll(f)
			_, _ = io.WriteString(w, `{"text":"heard `+string(b)+`"}`)
		case "/v1/embeddings":
			_, _ = io.WriteString(w, `{"model":"e","data":[{"index":1,"embedding":[2]},{"index":0,"embedding":[1]}],"usage":{"prompt_tokens":2}}`)
		case "/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"a","owned_by":"o","extra":1},{"id":""}]}`)
		default:
			w.WriteHeader(404)
			_, _ = io.WriteString(w, `{"error":"no such path"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL + "/v1/", Key: "k"}, &last
}

func TestChat(t *testing.T) {
	c, last := fake(t)
	r, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}},
		Extra: map[string]any{"reasoning": map[string]any{"effort": "low"}}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text() != "Hi" || r.Choices[0].Message.Reasoning != "think" || r.Usage.Cost == nil || *r.Usage.Cost != 0.001 {
		t.Fatalf("%+v", r)
	}
	if (*last)["reasoning"] == nil || (*last)["stream"] != nil {
		t.Fatalf("extra not merged or stream set: %v", *last)
	}
}

func TestChatStream(t *testing.T) {
	c, _ := fake(t)
	var pieces []string
	r, err := c.ChatStream(context.Background(), ChatRequest{Model: "m"}, func(d Delta) bool {
		pieces = append(pieces, d.Text+"|"+d.Reasoning)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text() != "Hello" || r.Choices[0].Message.Reasoning != "hmm" || r.Choices[0].FinishReason != "stop" || r.Usage.TotalTokens != 5 || r.ID != "c1" {
		t.Fatalf("%+v", r)
	}
	if strings.Join(pieces, ",") != "|hmm,Hel|,lo|" {
		t.Fatalf("pieces %v", pieces)
	}
	// Stopping early.
	r, err = c.ChatStream(context.Background(), ChatRequest{Model: "m"}, func(d Delta) bool { return d.Text == "" })
	if err != nil || r.Text() != "Hel" || r.Choices[0].FinishReason != "cancelled" {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestMedia(t *testing.T) {
	c, _ := fake(t)
	ctx := context.Background()
	img, err := c.Images(ctx, ImageRequest{Model: "i", Prompt: "a cat", ResponseFormat: "b64_json"})
	if err != nil || len(img.Images) != 2 || img.Images[0].ContentType != "image/png" || img.Images[1].URL != "https://x/y.png" {
		t.Fatalf("%+v %v", img, err)
	}
	sp, err := c.Speech(ctx, SpeechRequest{Model: "s", Input: "hello"})
	if err != nil || sp.ContentType != "audio/mpeg" || string(sp.Data) != "ID3audio" {
		t.Fatalf("%+v %v", sp, err)
	}
	tr, err := c.Transcribe(ctx, TranscribeRequest{Model: "w", Audio: []byte("words")})
	if err != nil || tr.Text != "heard words" {
		t.Fatalf("%+v %v", tr, err)
	}
	em, err := c.Embed(ctx, EmbedRequest{Model: "e", Input: []string{"a", "b"}})
	if err != nil || len(em.Vectors) != 2 || em.Vectors[0][0] != 1 || em.Vectors[1][0] != 2 {
		t.Fatalf("%+v %v", em, err)
	}
	ms, err := c.Models(ctx)
	if err != nil || len(ms) != 1 || ms[0].ID != "a" || !strings.Contains(string(ms[0].Raw), "extra") {
		t.Fatalf("%+v %v", ms, err)
	}
}

func TestErrors(t *testing.T) {
	c, _ := fake(t)
	var e *Error
	_, err := (&Client{BaseURL: c.BaseURL, Key: "nope"}).Chat(context.Background(), ChatRequest{Model: "m"})
	if !errors.As(err, &e) || e.Status != 401 || e.Message != "bad key" || e.Code != "auth" {
		t.Fatalf("%#v", err)
	}
	err = c.JSON(context.Background(), "GET", "nowhere", nil, nil)
	if !errors.As(err, &e) || e.Status != 404 || e.Message != "no such path" {
		t.Fatalf("%#v", err)
	}
}
