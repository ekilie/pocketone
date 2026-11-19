package s3fallback_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/pockethost/pocketone/ext/extkit/modetest"
	"github.com/pockethost/pocketone/ext/s3redirect/s3test"
)

// The stock pocketone binary as a multisite host (Pocker) whose `storage` section mounts uploads on a shared bucket:
// each isolate's files land in its own folder there, its Settings still show S3 off (no credentials), and downloads
// become signed redirects to that folder (S3 Fallback and S3 Redirect, both standard, with nothing wiring them
// together). An isolate that turns on its own S3 takes over with its own bucket, and one with the extension off keeps
// files on its own disk.
func TestModesHostSharedBucket(t *testing.T) {
	srv := s3test.Start(t, "shared", "own")
	top := modetest.Topology{
		Instances: []modetest.Inst{
			{ID: "a"},
			{ID: "b"},
			{ID: "off", Extensions: map[string]any{"storage": map[string]any{"enabled": false}}},
		},
		Hosts: []modetest.HostSpec{s3test.LoopbackHost(map[string]any{"storage": map[string]any{
			"backends": map[string]any{"shared": srv.Backend("shared", "eu/")},
			"mounts":   map[string]any{"storage": "shared"},
		}})},
	}
	modetest.Run(t, top, []modetest.Mode{modetest.Host}, func(t *testing.T, w *modetest.World) {
		for _, id := range []string{"a", "b"} {
			s := w.Instance(id)
			var set struct {
				S3 struct {
					Enabled bool   `json:"enabled"`
					Bucket  string `json:"bucket"`
				} `json:"s3"`
			}
			s.MustJSON(http.MethodGet, "/api/settings", s.JWT(), nil, 200, &set)
			if set.S3.Enabled || set.S3.Bucket != "" {
				t.Fatalf("%s: settings show S3 %+v; the shared bucket must stay invisible", id, set.S3)
			}

			colID, recID, name := s3test.Upload(t, s, s3test.PNG)
			for _, q := range []string{"", "?thumb=10x10"} {
				loc := s3test.Redirect(t, s, "/api/files/docs/"+recID+"/"+name+q)
				want := srv.URL + "/shared/eu/instances/" + id + "/storage/" + colID + "/" + recID + "/"
				if !strings.HasPrefix(loc.String(), want) {
					t.Fatalf("%s %q: redirect %s, want under %s", id, q, loc, want)
				}
				body, _ := s3test.Fetch(t, loc.String())
				if q == "" && !bytes.Equal(body, s3test.PNG) {
					t.Fatalf("%s: bucket served %d bytes, want the uploaded %d", id, len(body), len(s3test.PNG))
				}
			}
		}

		t.Run("own S3 takes over", func(t *testing.T) {
			s := w.Instance("a")
			s3test.UseOwnS3(t, s, srv, "own")
			colID, recID, name := s3test.Upload(t, s, s3test.PNG)
			loc := s3test.Redirect(t, s, "/api/files/docs/"+recID+"/"+name)
			if want := srv.URL + "/own/" + colID + "/" + recID + "/" + name; !strings.HasPrefix(loc.String(), want) {
				t.Fatalf("redirect %s, want %s (the instance's own bucket)", loc, want)
			}
			if body, _ := s3test.Fetch(t, loc.String()); !bytes.Equal(body, s3test.PNG) {
				t.Fatalf("own bucket served %d bytes", len(body))
			}
		})

		t.Run("extension off keeps local disk", func(t *testing.T) {
			s := w.Instance("off")
			_, recID, name := s3test.Upload(t, s, s3test.PNG)
			s3test.Streams(t, s, "/api/files/docs/"+recID+"/"+name, s3test.PNG)
		})
	})
}

// A single app gets S3 Fallback too (it's standard). Without a `storage` section nothing changes: files stay on local
// disk and stream. With one, the app's files go to the backend's root (no instances/<id>/ folder) and redirect there.
func TestModesSingleApp(t *testing.T) {
	srv := s3test.Start(t, "shared")
	top := modetest.Topology{Instances: []modetest.Inst{
		{ID: "plain"},
		{ID: "mounted", Extensions: map[string]any{"storage": map[string]any{
			"backends": map[string]any{"shared": srv.Backend("shared", "app/")},
			"mounts":   map[string]any{"storage": "shared"},
		}}},
	}}
	modetest.Run(t, top, []modetest.Mode{modetest.Single}, func(t *testing.T, w *modetest.World) {
		plain := w.Instance("plain")
		_, recID, name := s3test.Upload(t, plain, s3test.PNG)
		s3test.Streams(t, plain, "/api/files/docs/"+recID+"/"+name, s3test.PNG)

		s := w.Instance("mounted")
		colID, recID, name := s3test.Upload(t, s, s3test.PNG)
		loc := s3test.Redirect(t, s, "/api/files/docs/"+recID+"/"+name)
		if want := srv.URL + "/shared/app/storage/" + colID + "/" + recID + "/" + name; !strings.HasPrefix(loc.String(), want) {
			t.Fatalf("redirect %s, want %s", loc, want)
		}
		if body, _ := s3test.Fetch(t, loc.String()); !bytes.Equal(body, s3test.PNG) {
			t.Fatalf("bucket served %d bytes", len(body))
		}
	})
}
