package vultr

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestModelsAndJobs(t *testing.T) {
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"z","name":"Z","output_modalities":[{"type":"image","streaming":true,
				"supported_parameters":{"size":{"type":"unknown"}},"pricing":[{"type":"completion","unit":"megapixel","cost_usd":"0.02"}]}]}]}`)
		case "/v1/music":
			_, _ = io.WriteString(w, `{"id":"m1","object":"music","model":"a","status":"pending","progress":0}`)
		case "/v1/music/m1":
			if polls.Add(1) < 2 {
				_, _ = io.WriteString(w, `{"id":"m1","object":"music","status":"in_progress","progress":50}`)
				return
			}
			_, _ = io.WriteString(w, `{"id":"m1","object":"music","status":"completed","progress":100,"outputs":[{"index":0,"content_type":"audio/mpeg"}]}`)
		case "/v1/music/m1/content":
			if r.URL.Query().Get("index") != "0" {
				w.WriteHeader(400)
				return
			}
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = io.WriteString(w, "ID3music")
		case "/v1/videos":
			_, _ = io.WriteString(w, `{"id":"v1","object":"video","status":"failed","error":"too long","error_code":"invalid_request"}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := New(srv.URL+"/v1", "k")
	ctx := context.Background()

	ms, err := c.Models(ctx)
	if err != nil || len(ms) != 1 || !ms[0].Makes("image") || ms[0].Outputs[0].Pricing[0].USD != "0.02" || ms[0].Outputs[0].Params["size"] == nil {
		t.Fatalf("%+v %v", ms, err)
	}

	j, err := c.Music(ctx, MusicRequest{Model: "a", Prompt: "lullaby", Duration: 10, Extra: map[string]any{"quality": "turbo"}})
	if err != nil || j.ID != "m1" || j.Done() {
		t.Fatalf("%+v %v", j, err)
	}
	if j, err = c.Wait(ctx, "music", j, time.Millisecond); err != nil || j.Status != "completed" || len(j.Outputs) != 1 {
		t.Fatalf("%+v %v", j, err)
	}
	f, err := c.Content(ctx, "music", j.ID, 0)
	if err != nil || string(f.Data) != "ID3music" || f.ContentType != "audio/mpeg" {
		t.Fatalf("%+v %v", f, err)
	}

	v, err := c.Video(ctx, VideoRequest{Model: "v", Prompt: "sea"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Wait(ctx, "video", v, time.Millisecond); err == nil || v.Err() == nil {
		t.Fatal("a failed job should be an error")
	}
}
