package openrouter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pocketone.dev/ext/llm/providers/oai"
)

func TestDriver(t *testing.T) {
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nxxxx"))
	var last map[string]any
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last = nil
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			_ = json.NewDecoder(r.Body).Decode(&last)
		}
		switch r.URL.Path {
		case "/api/v1/models":
			if r.URL.Query().Get("output_modalities") != "all" {
				w.WriteHeader(400)
				return
			}
			_, _ = io.WriteString(w, `{"data":[{"id":"a/img","architecture":{"output_modalities":["image"]},"endpoints":["/api/v1/images"],
				"pricing":{"image_output":"0.04"},"reasoning":{"mandatory":false,"default_enabled":true}}]}`)
		case "/api/v1/images":
			_, _ = io.WriteString(w, `{"created":1,"data":[{"b64_json":"`+png+`","media_type":"image/png"}],"usage":{"cost":0.04}}`)
		case "/api/v1/chat/completions":
			_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"here","images":[{"image_url":{"url":"data:image/png;base64,`+png+`"}}]},"finish_reason":"stop"}]}`)
		case "/api/v1/audio/speech":
			if last["response_format"] != "mp3" {
				w.WriteHeader(400)
				return
			}
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = io.WriteString(w, "ID3")
		case "/api/v1/videos":
			w.WriteHeader(202)
			_, _ = io.WriteString(w, `{"id":"gen-vid-1","polling_url":"/api/v1/videos/gen-vid-1","status":"pending"}`)
		case "/api/v1/videos/gen-vid-1":
			polls++
			if polls < 2 {
				_, _ = io.WriteString(w, `{"id":"gen-vid-1","status":"in_progress","progress":40}`)
				return
			}
			_, _ = io.WriteString(w, `{"id":"gen-vid-1","status":"completed","unsigned_urls":["/api/v1/videos/gen-vid-1/content?index=0"]}`)
		case "/api/v1/videos/gen-vid-1/content":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = io.WriteString(w, "mp4data")
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := New(srv.URL+"/api/v1", "k")
	ctx := context.Background()

	ms, err := c.Models(ctx, "all")
	if err != nil || len(ms) != 1 || !ms[0].Makes("image") || !ms[0].Serves("/api/v1/images") || !ms[0].Reasoning.DefaultEnabled {
		t.Fatalf("%+v %v", ms, err)
	}
	im, err := c.Images(ctx, oai.ImageRequest{Model: "a/img", Prompt: "cat"})
	if err != nil || len(im.Images) != 1 || im.Images[0].ContentType != "image/png" {
		t.Fatalf("%+v %v", im, err)
	}
	r, files, err := c.ChatImages(ctx, oai.ChatRequest{Model: "g/flash-image", Messages: []oai.Message{{Role: "user", Content: "cat"}}})
	if err != nil || r.Text() != "here" || len(files) != 1 || files[0].ContentType != "image/png" || last["modalities"] == nil {
		t.Fatalf("%+v %+v %v %v", r, files, err, last)
	}
	sp, err := c.Speech(ctx, oai.SpeechRequest{Model: "s", Input: "hi"})
	if err != nil || string(sp.Data) != "ID3" {
		t.Fatalf("%+v %v", sp, err)
	}
	j, err := c.Video(ctx, VideoRequest{Model: "v", Prompt: "sea", Duration: 4})
	if err != nil || j.ID != "gen-vid-1" {
		t.Fatalf("%+v %v", j, err)
	}
	if j, err = c.Wait(ctx, j, time.Millisecond); err != nil || j.Status != "completed" {
		t.Fatalf("%+v %v", j, err)
	}
	f, err := c.Content(ctx, j.ID, 0)
	if err != nil || string(f.Data) != "mp4data" || f.ContentType != "video/mp4" {
		t.Fatalf("%+v %v", f, err)
	}
}
