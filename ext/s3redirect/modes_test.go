package s3redirect_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pockethost/pocketone/ext/extkit/modetest"
	"github.com/pockethost/pocketone/ext/s3redirect/s3test"
)

// The pocketone binary, as a single app and as an isolate on a multisite host (Pocker), with the app's own S3 in
// Settings → Files storage: downloads answer 302 to a URL signed for exactly that object, the bucket serves the file
// from it (the fake bucket checks the signature as S3 does), thumbs too, with the disposition signed in, and the URL
// lives as long as the configured TTL.
func TestModesRedirectsToOwnS3(t *testing.T) {
	srv := s3test.Start(t, "own")
	top := modetest.Topology{
		Instances: []modetest.Inst{{ID: "app", Extensions: map[string]any{"s3redirect": map[string]any{"ttl": "2m"}}}},
		Hosts:     []modetest.HostSpec{s3test.LoopbackHost(nil)},
	}
	modetest.Run(t, top, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")
		s3test.UseOwnS3(t, s, srv, "own")
		colID, recID, name := s3test.Upload(t, s, s3test.PNG)
		before := srv.Presigned()

		for _, q := range []string{"", "?thumb=10x10"} {
			loc := s3test.Redirect(t, s, "/api/files/docs/"+recID+"/"+name+q)
			if !strings.HasPrefix(loc.String(), srv.URL+"/own/"+colID+"/"+recID+"/") {
				t.Fatalf("%q: redirect %s, want the own bucket's object", q, loc)
			}
			if got := loc.Query().Get("X-Amz-Expires"); got != "120" {
				t.Fatalf("%q: X-Amz-Expires %s, want 120 (ttl 2m)", q, got)
			}
			body, hdr := s3test.Fetch(t, loc.String())
			if q == "" && !bytes.Equal(body, s3test.PNG) {
				t.Fatalf("bucket served %d bytes, want the uploaded %d", len(body), len(s3test.PNG))
			}
			if q != "" && !bytes.HasPrefix(body, s3test.PNG[:8]) {
				t.Fatalf("thumb isn't a PNG: %q", body[:min(16, len(body))])
			}
			if d := hdr.Get("Content-Disposition"); !strings.HasPrefix(d, "inline") {
				t.Fatalf("%q: Content-Disposition %q, want inline (signed in as response-content-disposition)", q, d)
			}
		}
		if n := srv.Presigned() - before; n != 2 {
			t.Fatalf("bucket served %d presigned downloads, want 2", n)
		}
	})
}

// `s3redirect: {enabled: false}` (a single app's section, an isolate's extensions.s3redirect) streams files through
// the app even with S3 on.
func TestModesOffStreams(t *testing.T) {
	srv := s3test.Start(t, "own")
	top := modetest.Topology{
		Instances: []modetest.Inst{{ID: "app", Extensions: map[string]any{"s3redirect": map[string]any{"enabled": false}}}},
		Hosts:     []modetest.HostSpec{s3test.LoopbackHost(nil)},
	}
	modetest.Run(t, top, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")
		s3test.UseOwnS3(t, s, srv, "own")
		_, recID, name := s3test.Upload(t, s, s3test.PNG)
		s3test.Streams(t, s, "/api/files/docs/"+recID+"/"+name, s3test.PNG)
	})
}

// The single app's --s3Redirect=false flag streams files through the app, and wins over the config file.
func TestModesFlagOffStreams(t *testing.T) {
	srv := s3test.Start(t, "own")
	top := modetest.Topology{Instances: []modetest.Inst{{ID: "app", Args: []string{"--s3Redirect=false"},
		Extensions: map[string]any{"s3redirect": map[string]any{"enabled": true}}}}}
	modetest.Run(t, top, []modetest.Mode{modetest.Single}, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")
		s3test.UseOwnS3(t, s, srv, "own")
		_, recID, name := s3test.Upload(t, s, s3test.PNG)
		s3test.Streams(t, s, "/api/files/docs/"+recID+"/"+name, s3test.PNG)
	})
}
