package vultr

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAlignAndCatalog(t *testing.T) {
	var got map[string]string
	var file []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/audio/alignments":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			got = map[string]string{}
			for k, v := range r.MultipartForm.Value {
				got[k] = v[0]
			}
			f, h, _ := r.FormFile("file")
			file, _ = io.ReadAll(f)
			got["filename"] = h.Filename
			_, _ = io.WriteString(w, `{"duration":2.5,"separated":false,
				"lines":[{"index":0,"section":0,"tag":null,"text":"Good night","start":0.2,"end":1.1,"score":0.99,"certain":true}],
				"words":[{"line":0,"text":"Good","start":0.2,"end":0.5,"score":0.98},{"line":0,"text":"night","start":0.6,"end":1.1,"score":0.99}],
				"unaligned":[],"usage":{"type":"audio","seconds":2.5,"cost":0.0001}}`)
		case "/v1/models/all":
			_, _ = io.WriteString(w, `{"chat":[{"id":"chat-1","features":["Chat"]}],"audio":[{"id":"aligner-1","features":["AudioAlignment"]}]}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := New(srv.URL+"/v1", "k")
	a, err := c.Align(context.Background(), AlignRequest{Model: "aligner-1", Audio: []byte("mp3"), Filename: "s.mp3", Text: "Good night", Mode: "speech"})
	if err != nil || a.Duration != 2.5 || len(a.Words) != 2 || a.Lines[0].Text != "Good night" || a.Usage.Cost == nil {
		t.Fatalf("align: %+v %v", a, err)
	}
	if got["model"] != "aligner-1" || got["mode"] != "speech" || got["text"] != "Good night" || got["filename"] != "s.mp3" ||
		string(file) != "mp3" || got["language"] != "" {
		t.Fatalf("sent: %v %q", got, file)
	}
	cat, err := c.Catalog(context.Background())
	if err != nil || len(cat) != 2 {
		t.Fatalf("catalog: %+v %v", cat, err)
	}
	n := 0
	for _, m := range cat {
		if m.Has("AudioAlignment") {
			n++
			if m.ID != "aligner-1" {
				t.Fatal(m.ID)
			}
		}
	}
	if n != 1 {
		t.Fatal("aligner not found")
	}
}
