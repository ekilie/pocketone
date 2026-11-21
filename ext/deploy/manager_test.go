package deploy

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
	"pocketone.dev/ext/contracts/instance/source/v1/sourcetest"
	"strings"
	"testing"
	"time"

	"pocketone.dev/ext/pocker"
)

// TestWithPockerManager deploys to a real PocketBase app run by Pocker: hooks change after a restart, a broken hook is
// reported as startError, and pb_public-only deploys don't restart.
func TestWithPockerManager(t *testing.T) {
	root := t.TempDir()
	pbRoot := filepath.Join(root, "instances")
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := pocker.NewManager(sourcetest.ScopeOf(pocker.Instance{ID: "inst1", Hosts: []string{"myapp.example.com"}}), pocker.ManagerOptions{Standard: testStandard, PBRoot: pbRoot, Logger: quiet})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Run(ctx)
	defer mgr.Shutdown()

	inst := Instance{ID: "inst1", Subdomain: "myapp", OwnerID: "user1", Power: true}
	be := &fakeBackend{insts: map[string]Instance{"myapp.example.com": inst},
		pats: map[string]fakeTok{testPAT: {UserID: "user1", Scopes: []string{"read", "write"}}}}
	ifs := newManagerFiles(mgr, pbRoot, filepath.Join(root, "deploys"))
	api := New(Options{Prefix: Prefix, Backend: be, Files: func() fsv1.Files { return ifs }, Logger: quiet})
	srv := httptest.NewServer(api.Wrap(mgr))
	defer srv.Close()
	f := &apiFixture{t: t, root: root, be: be, api: api, srv: srv}

	get := func(path string) string {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		req.Host = "myapp.example.com"
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return strings.TrimSpace(string(b))
	}
	hook := func(v string) string { return `routerAdd("GET", "/v", (e) => e.string(200, "` + v + `"))` }

	r := f.deploy(map[string]string{"pb_hooks/main.pb.js": hook("v1"), "pb_public/index.html": "one"}, nil)
	if r.status != 200 || r.body["wasRunning"] != false {
		t.Fatalf("first: %s", r.raw)
	}
	deadline := time.Now().Add(10 * time.Second)
	for get("/v") != "v1" {
		if time.Now().After(deadline) {
			t.Fatal("v1 never served")
		}
		time.Sleep(50 * time.Millisecond)
	}

	waitFor := func(v string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !strings.HasPrefix(get("/v"), v) {
			if time.Now().After(deadline) {
				t.Fatalf("%s never served (got %q)", v, get("/v"))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// hooks: deploy only swaps the files in; Pocker's watcher reloads the app
	r = f.deploy(map[string]string{"pb_hooks/main.pb.js": hook("v2"), "pb_public/index.html": "one"}, nil)
	if r.status != 200 || r.body["restarted"] != false || r.body["restartNeeded"] != nil || !mgr.Running("inst1") {
		t.Fatalf("second: %s", r.raw)
	}
	waitFor("v2")

	r = f.deploy(map[string]string{"pb_hooks/main.pb.js": hook("v2"), "pb_public/index.html": "two"}, nil)
	if r.body["restarted"] != false || !mgr.Running("inst1") || get("/index.html") != "two" {
		t.Fatalf("pb_public only: %s", r.raw)
	}

	// a broken hook on a production instance: the watcher restarts it, and it doesn't start until fixed
	r = f.deploy(map[string]string{"pb_hooks/main.pb.js": "this is not js (", "pb_public/index.html": "two"}, nil)
	if r.status != 200 || r.body["restarted"] != false {
		t.Fatalf("broken: %s", r.raw)
	}
	waitFor("instance failed to start")

	r = f.deploy(map[string]string{"pb_hooks/main.pb.js": hook("v3"), "pb_public/index.html": "two"}, nil)
	if r.status != 200 {
		t.Fatalf("fixed: %s", r.raw)
	}
	waitFor("v3")

	// a migration waits for a restart, which the client asks for
	files := map[string]string{"pb_hooks/main.pb.js": hook("v3"), "pb_public/index.html": "two",
		"pb_migrations/1_posts.js": `migrate((app) => { app.save(new Collection({name: "posts", type: "base"})) })`}
	r = f.deploy(files, nil)
	if r.status != 200 || r.body["restartNeeded"] != true || r.body["restarted"] != false {
		t.Fatalf("migration: %s", r.raw)
	}
	// asked for on the next commit (remote/v1's restart is the other way)
	files["pb_migrations/2_tags.js"] = `migrate((app) => { app.save(new Collection({name: "tags", type: "base"})) })`
	r = f.deploy(files, map[string]any{"restart": true})
	if r.status != 200 || r.body["restarted"] != true || !mgr.Running("inst1") {
		t.Fatalf("restart: %d %s", r.status, r.raw)
	}
	waitFor("v3")
}
