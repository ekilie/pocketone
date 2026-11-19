package s3test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"testing"
	"time"

	"pocketone.dev/ext/extkit/modetest"
)

// Helpers for mode tests (ext/extkit/modetest) of file downloads: an app's files, uploaded and fetched over HTTP like a
// client would.

// LoopbackHost is the default multisite host, allowed to reach a [Server] on 127.0.0.1 (Pocker blocks private
// networks for instances otherwise).
func LoopbackHost(config map[string]any) modetest.HostSpec {
	cfg := map[string]any{"pocker": map[string]any{"root": "./pb_root", "egress_allow": []string{"127.0.0.0/8"}}}
	for k, v := range config {
		cfg[k] = v
	}
	return modetest.HostSpec{ID: "host", Config: cfg}
}

// Backend is srv's bucket as a backend of ext/s3-fallback's `storage` section.
func (s *Server) Backend(bucket, prefix string) map[string]any {
	return map[string]any{"endpoint": s.URL, "bucket": bucket, "region": s.Region, "access_key": s.AccessKey,
		"secret": s.Secret, "prefix": prefix, "force_path_style": true}
}

// UseOwnS3 turns on the app's own S3 (Settings → Files storage) on srv's bucket.
func UseOwnS3(t testing.TB, s *modetest.Site, srv *Server, bucket string) {
	t.Helper()
	s.MustJSON(http.MethodPatch, "/api/settings", s.JWT(), map[string]any{"s3": map[string]any{
		"enabled": true, "bucket": bucket, "region": srv.Region, "endpoint": srv.URL,
		"accessKey": srv.AccessKey, "secret": srv.Secret, "forcePathStyle": true,
	}}, 200, nil)
}

// Upload creates a public "docs" collection with a file field "img" (thumbs 10x10) if it isn't there, uploads data
// as pic.png in a new record, and returns the collection id, record id and stored file name.
func Upload(t testing.TB, s *modetest.Site, data []byte) (colID, recID, name string) {
	t.Helper()
	var col struct {
		ID string `json:"id"`
	}
	if code, b := s.Raw(http.MethodGet, "/api/collections/docs", s.JWT(), nil); code == 200 {
		_ = json.Unmarshal(b, &col)
	} else {
		s.MustJSON(http.MethodPost, "/api/collections", s.JWT(), map[string]any{
			"name": "docs", "type": "base", "viewRule": "", "listRule": "",
			"fields": []map[string]any{{"name": "img", "type": "file", "maxSelect": 1, "maxSize": 1 << 20, "thumbs": []string{"10x10"}}},
		}, 200, &col)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("img", "pic.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, s.URL+"/api/collections/docs/records", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", s.JWT())
	res, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("%s upload: %d %s", s.ID, res.StatusCode, b)
	}
	var rec struct {
		ID  string `json:"id"`
		Img string `json:"img"`
	}
	if err := json.Unmarshal(b, &rec); err != nil || rec.Img == "" {
		t.Fatalf("%s upload: %s", s.ID, b)
	}
	return col.ID, rec.ID, rec.Img
}

// Redirect GETs path from the app without following redirects, fails unless it's a 302 that keeps the referrer
// private, and returns its Location.
func Redirect(t testing.TB, s *modetest.Site, path string) *url.URL {
	t.Helper()
	res, err := s.Client().Get(s.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("%s GET %s: %d, want 302\n%s", s.ID, path, res.StatusCode, b)
	}
	if rp := res.Header.Get("Referrer-Policy"); rp != "no-referrer" {
		t.Fatalf("%s GET %s: Referrer-Policy %q", s.ID, path, rp)
	}
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// Streams fails unless GET path answers 200 from the app itself with want.
func Streams(t testing.TB, s *modetest.Site, path string, want []byte) {
	t.Helper()
	res, err := s.Client().Get(s.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !bytes.Equal(b, want) {
		t.Fatalf("%s GET %s: %d (%d bytes, Location %q), want the file streamed (200, %d bytes)", s.ID, path,
			res.StatusCode, len(b), res.Header.Get("Location"), len(want))
	}
}

// Fetch GETs a bucket URL as a browser following the redirect would, and fails unless it's 200.
func Fetch(t testing.TB, u string) ([]byte, http.Header) {
	t.Helper()
	c := &http.Client{Timeout: 5 * time.Second}
	res, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("bucket GET %s: %d %s", u, res.StatusCode, b)
	}
	return b, res.Header
}

// PNG is a 20x20 PNG, big enough for a 10x10 thumb.
var PNG = func() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for x := range 20 {
		for y := range 20 {
			img.Set(x, y, color.RGBA{uint8(x * 12), uint8(y * 12), 200, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}()
