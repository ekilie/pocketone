package s3test_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/tools/filesystem"
	"pocketone.dev/ext/s3redirect/s3test"
	"pocketone.dev/ext/s3redirect/sigv4"
)

// The fake bucket serves PocketBase's own S3 client, and serves a presigned GET only when it's signed for exactly
// that object, by its key, and not expired: a test that follows a wrong redirect fails.
func TestPresignedChecks(t *testing.T) {
	srv := s3test.Start(t, "b")
	fsys, err := filesystem.NewS3("b", srv.Region, srv.URL, srv.AccessKey, srv.Secret, true)
	if err != nil {
		t.Fatal(err)
	}
	defer fsys.Close()
	if err := fsys.Upload([]byte("hello"), "dir/a b.txt"); err != nil {
		t.Fatalf("PocketBase's client can't upload: %v", err)
	}

	cfg := sigv4.Config{Endpoint: srv.URL, Bucket: "b", Region: srv.Region, AccessKey: srv.AccessKey, SecretKey: srv.Secret, PathStyle: true}
	get := func(u string) (int, string) {
		res, err := http.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var b bytes.Buffer
		_, _ = b.ReadFrom(res.Body)
		return res.StatusCode, b.String()
	}

	good, _ := cfg.PresignGet("dir/a b.txt", time.Minute, time.Now(), nil)
	if code, body := get(good); code != 200 || body != "hello" {
		t.Fatalf("signed URL: %d %q", code, body)
	}

	other, _ := cfg.PresignGet("dir/other.txt", time.Minute, time.Now(), nil)
	swapped := strings.Replace(good, "/dir/a%20b.txt?", "/dir/other.txt?", 1)
	if swapped == good {
		t.Fatalf("test setup: path not found in %s", good)
	}
	expired, _ := cfg.PresignGet("dir/a b.txt", time.Second, time.Now().Add(-time.Hour), nil)
	wrongKey := cfg
	wrongKey.SecretKey = "nope"
	forged, _ := wrongKey.PresignGet("dir/a b.txt", time.Minute, time.Now(), nil)

	for name, u := range map[string]string{
		"signed for another object": swapped,
		"expired":                   expired,
		"wrong secret":              forged,
		"unsigned":                  srv.URL + "/b/dir/a%20b.txt",
	} {
		if code, body := get(u); code != http.StatusForbidden {
			t.Errorf("%s: %d %q, want 403", name, code, body)
		}
	}
	if code, _ := get(other); code != http.StatusNotFound {
		t.Errorf("valid signature for a missing object: %d, want 404", code)
	}
	if n := srv.Presigned(); n != 2 { // good + other passed the check
		t.Errorf("Presigned() = %d, want 2", n)
	}
}
