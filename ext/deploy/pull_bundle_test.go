package deploy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	remoteclient "pocketone.dev/ext/contracts/remote/v1/remoteclient"
)

// countingClient is a deploy client for f's instance behind a wrapper that counts requests by path and can answer 404
// for some paths (an older server).
func countingClient(t *testing.T, f *apiFixture, missing ...string) (*Client, func() map[string]int) {
	t.Helper()
	var mu sync.Mutex
	counts := map[string]int{}
	inner := f.api.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(299) }))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, Prefix)
		mu.Lock()
		counts[p]++
		mu.Unlock()
		for _, m := range missing {
			if p == m {
				http.Error(w, `{"message":"Not found"}`, http.StatusNotFound)
				return
			}
		}
		r.Host = "myapp.example.com"
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	rc := &remoteclient.Client{Remote: &remoteclient.Remote{Name: "test", URL: srv.URL}, Token: testPAT,
		HTTP: remoteclient.NewHTTPClient()}
	return &Client{Client: rc, Prefix: Prefix}, func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := map[string]int{}
		for k, v := range counts {
			out[k] = v
		}
		clear(counts)
		return out
	}
}

func total(m map[string]int) (n int) {
	for _, v := range m {
		n += v
	}
	return n
}

// A pull is the tree in one request and the files in one bundle, whatever their number; a later pull fetches only what
// changed; the deploy base comes from the same tree.
func TestPullInBundles(t *testing.T) {
	f := newAPIFixture(t)
	for i := range 40 {
		f.put(fmt.Sprintf("pb_pages/p%02d/index.ejs", i%8)+fmt.Sprint(i), fmt.Sprintf("page %d", i))
	}
	f.put("pb_hooks/main.pb.js", "hook")
	f.put("package.json", `{"dependencies":{}}`)
	c, counts := countingClient(t, f)
	dir := t.TempDir()

	n, err := Pull(context.Background(), c, PullOptions{Dir: dir})
	if err != nil || n != 42 {
		t.Fatalf("pull: %d %v", n, err)
	}
	got := counts()
	if got["/tree"] != 1 || got["/files/bundle"] != 1 || total(got) != 2 {
		t.Fatalf("requests for a 42-file pull: %v, want one tree and one bundle", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "pb_hooks", "main.pb.js")); string(b) != "hook" {
		t.Fatalf("main.pb.js = %q", b)
	}
	state, err := os.ReadFile(filepath.Join(dir, syncStateDir, syncStateFile))
	if err != nil || !strings.Contains(string(state), "pb_hooks/main.pb.js") {
		t.Fatalf("deploy base: %v %s", err, state)
	}

	// one file changes on the server: the next pull fetches that file only
	f.put("pb_hooks/main.pb.js", "hook v2")
	if n, err = Pull(context.Background(), c, PullOptions{Dir: dir}); err != nil || n != 42 {
		t.Fatalf("second pull: %d %v", n, err)
	}
	got = counts()
	if got["/files/bundle"] != 1 || got["/files/raw"] != 0 {
		t.Fatalf("requests for one changed file: %v", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "pb_hooks", "main.pb.js")); string(b) != "hook v2" {
		t.Fatalf("changed file = %q", b)
	}

	// nothing changed: the tree only
	if _, err = Pull(context.Background(), c, PullOptions{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if got = counts(); got["/files/bundle"] != 0 || got["/files/raw"] != 0 {
		t.Fatalf("requests with nothing to fetch: %v", got)
	}
}

// An older server without bundles (or without deep trees) still pulls, file by file.
func TestPullFromAnOlderServer(t *testing.T) {
	f := newAPIFixture(t)
	f.put("pb_pages/a/index.ejs", "a")
	f.put("pb_pages/b/index.ejs", "b")
	f.put("pb_hooks/main.pb.js", "hook")
	c, counts := countingClient(t, f, "/files/bundle")
	dir := t.TempDir()
	if n, err := Pull(context.Background(), c, PullOptions{Dir: dir}); err != nil || n != 3 {
		t.Fatalf("pull: %d %v", n, err)
	}
	if got := counts(); got["/files/raw"] != 3 {
		t.Fatalf("requests: %v, want one raw download per file", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "pb_pages", "b", "index.ejs")); string(b) != "b" {
		t.Fatalf("b = %q", b)
	}

	// no tree at all: the folder listing and file-by-file pull, as before
	c2, _ := countingClient(t, f, "/tree", "/files/bundle")
	dir2 := t.TempDir()
	if n, err := Pull(context.Background(), c2, PullOptions{Dir: dir2}); err != nil || n != 3 {
		t.Fatalf("pull without a tree: %d %v", n, err)
	}
}
