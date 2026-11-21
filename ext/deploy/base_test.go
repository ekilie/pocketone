package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A deploy stops when the remote changed since the last deploy, pull or dev (a drive edit), lists what it would
// overwrite, and goes ahead with Force. Changes the deploy wouldn't undo don't stop it.
func TestDeployStopsOnServerChanges(t *testing.T) {
	r := newP1Remote(t)
	ctx := context.Background()
	c := r.client(t, r.token(t, "_superusers", "test@example.com", ScopeDeploy, "remote", "remote:read"))
	project := t.TempDir()
	write := func(root, rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(root, rel string) string {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "<missing>"
		}
		return string(b)
	}
	deploy := func(force bool) error {
		_, err := Deploy(ctx, c, DeployOptions{CollectOptions: CollectOptions{Dir: project}, Force: force})
		return err
	}

	// a server-only file before the first deploy: no base yet, so the first deploy overwrites as before (CI)
	write(r.dir, "pb_public/old.html", "left from before")
	write(project, "pb_pages/index.ejs", "page v1")
	write(project, "pb_hooks/a.pb.js", "hook v1")
	write(project, "pb_public/gone.html", "to delete on the server")
	if err := deploy(false); err != nil {
		t.Fatalf("first deploy: %v", err)
	}
	if got := read(r.dir, "pb_public/old.html"); got != "<missing>" {
		t.Fatalf("first deploy left old.html: %q", got)
	}

	// a local change alone deploys
	write(project, "pb_pages/index.ejs", "page v2")
	if err := deploy(false); err != nil {
		t.Fatalf("local change: %v", err)
	}

	// drive edits: a changed page, a new file, a deleted file, and one change the local copy already has
	write(r.dir, "pb_pages/index.ejs", "page edited on the drive")
	write(r.dir, "pb_public/new.html", "made on the drive")
	os.Remove(filepath.Join(r.dir, "pb_public/gone.html"))
	write(r.dir, "pb_hooks/a.pb.js", "hook v2")
	write(project, "pb_hooks/a.pb.js", "hook v2")

	err := deploy(false)
	var sc *ServerChangesError
	if !errors.As(err, &sc) {
		t.Fatalf("deploy over drive edits: %v, want ServerChangesError", err)
	}
	want := []ServerChange{
		{Path: "pb_pages/index.ejs", Kind: "changed"},
		{Path: "pb_public/gone.html", Kind: "deleted"},
		{Path: "pb_public/new.html", Kind: "added"},
	}
	if !reflect.DeepEqual(sc.Changes, want) {
		t.Fatalf("changes = %+v, want %+v", sc.Changes, want)
	}
	if got := read(r.dir, "pb_pages/index.ejs"); got != "page edited on the drive" {
		t.Fatalf("a stopped deploy changed the server: %q", got)
	}

	// pulling the page records it; the other two still stop the deploy
	if _, err := Pull(ctx, c, PullOptions{Dir: project, Paths: []string{"pb_pages"}}); err != nil {
		t.Fatal(err)
	}
	if !errors.As(deploy(false), &sc) || len(sc.Changes) != 2 {
		t.Fatalf("after pulling pb_pages: %+v", sc)
	}

	// --force overwrites, and the next deploy has nothing to stop on
	if err := deploy(true); err != nil {
		t.Fatalf("forced deploy: %v", err)
	}
	if got := read(r.dir, "pb_public/new.html"); got != "<missing>" {
		t.Fatalf("forced deploy left new.html: %q", got)
	}
	if got := read(r.dir, "pb_public/gone.html"); got != "to delete on the server" {
		t.Fatalf("forced deploy didn't restore gone.html: %q", got)
	}
	write(r.dir, "pb_public/new.html", "made again")
	os.Remove(filepath.Join(r.dir, "pb_public/new.html"))
	if err := deploy(false); err != nil {
		t.Fatalf("deploy after force: %v", err)
	}

	// dev reads the same base: no first-run question after a deploy
	s, err := NewSyncer(c, SyncOptions{CollectOptions: CollectOptions{Dir: project}})
	if err != nil {
		t.Fatal(err)
	}
	if s.firstRun {
		t.Fatal("dev doesn't see the deploy's base")
	}
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
}
