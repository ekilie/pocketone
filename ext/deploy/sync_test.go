package deploy

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pockethost/pocketone/ext/deploy/internal/glob"
)

func TestPlanSync(t *testing.T) {
	local := map[string]string{
		"same":        "a",
		"localEdit":   "L",
		"remoteEdit":  "b",
		"bothEdit":    "L",
		"localNew":    "n",
		"remoteDel":   "c",
		"delModified": "d", // local unchanged... remote modified it, local deleted? see below
		"firstDiff":   "x",
		"localMod":    "L", // modified here, deleted there
	}
	delete(local, "delModified") // deleted here
	remote := map[string]string{
		"same":        "a",
		"localEdit":   "b",
		"remoteEdit":  "R",
		"bothEdit":    "R",
		"remoteNew":   "n",
		"localDel":    "e",
		"delModified": "R",
		"firstDiff":   "y",
	}
	base := map[string]string{
		"same": "a", "localEdit": "b", "remoteEdit": "b", "bothEdit": "b", "remoteDel": "c", "localDel": "e",
		"delModified": "d", "goneBoth": "z", "localMod": "m",
	}
	p := planSync(local, remote, base)
	want := syncPlan{
		push:       []string{"localEdit", "localMod", "localNew"},
		pushDelete: []string{"localDel"},
		pull:       []string{"delModified", "remoteEdit", "remoteNew"},
		pullDelete: []string{"remoteDel"},
		conflicts:  []string{"bothEdit"},
		firstDiffs: []string{"firstDiff"},
		forget:     []string{"goneBoth"},
	}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("got  %+v\nwant %+v", p, want)
	}
}

func TestBuildTree(t *testing.T) {
	files := map[string]currentFile{
		"pb_hooks/a.js":          {hash: "1"},
		"pb_public/x/y/z.html":   {hash: "2"},
		"pb_public/index.html":   {hash: "3"},
		"package.json":           {hash: "4"},
		"pb_public/x/other.html": {hash: "5"},
	}
	before := buildTree(files)
	files["pb_public/x/y/z.html"] = currentFile{hash: "changed"}
	after := buildTree(files)
	for _, p := range []string{"", "pb_public", "pb_public/x", "pb_public/x/y"} {
		if before[p].hash == after[p].hash {
			t.Errorf("%q hash didn't change", p)
		}
	}
	if before["pb_hooks"].hash != after["pb_hooks"].hash {
		t.Error("pb_hooks hash changed")
	}
	if got := len(after[""].entries("", after)); got != 3 {
		t.Errorf("root entries %d, want 3", got)
	}
}

func TestReadIgnore(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".pocketoneignore")
	os.WriteFile(p, []byte("# comment\n\ngenerated/\n/pb_public/cache\n*.tmp\n"), 0o644)
	pats, err := readIgnore(p)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		"pb_public/generated/a.txt": true,
		"pb_public/cache/x":         true,
		"pb_hooks/cache/x":          false,
		"pb_hooks/a.tmp":            true,
		"pb_hooks/a.js":             false,
	} {
		if got := glob.Any(pats, path); got != want {
			t.Errorf("%s: %v, want %v", path, got, want)
		}
	}
}

func TestSyncP1(t *testing.T) {
	r := newP1Remote(t)
	ctx := context.Background()
	c := r.client(t, r.token(t, "_superusers", "test@example.com", ScopeDeploy))
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
	expect := func(root, rel, want string) {
		t.Helper()
		if got := read(root, rel); got != want {
			t.Fatalf("%s/%s = %q, want %q", filepath.Base(root), rel, got, want)
		}
	}

	write(project, "pb_hooks/a.pb.js", "hook v1")
	write(project, "pb_pages/index.ejs", "page v1")
	write(project, "pb_public/index.html", "local index")
	write(project, "pb_data/data.db", "local data")
	write(project, ".pocketoneignore", "generated/\n")
	write(r.dir, "pb_public/index.html", "server index")
	write(r.dir, "pb_public/server.html", "made on the server")

	s, err := NewSyncer(c, SyncOptions{CollectOptions: CollectOptions{Dir: project}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Sync(ctx); err == nil {
		t.Fatal("first sync with differing files and no --prefer should fail")
	}
	s.opts.Prefer = "local"
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	expect(r.dir, "pb_hooks/a.pb.js", "hook v1")
	expect(r.dir, "pb_pages/index.ejs", "page v1")
	expect(r.dir, "pb_public/index.html", "local index")
	expect(r.dir, "pb_data/data.db", "<missing>")
	expect(project, "pb_public/server.html", "made on the server")
	// sync never restarts by itself
	if n := r.restarts.Load(); n != 0 {
		t.Fatalf("first sync restarts = %d, want 0", n)
	}

	// a local edit goes up; the partial commit leaves the server's other files alone
	write(project, "pb_public/index.html", "local index v2")
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	expect(r.dir, "pb_public/index.html", "local index v2")
	expect(r.dir, "pb_public/server.html", "made on the server")

	// nothing changed: the poll sees no change
	if changed, err := s.RemoteChanged(ctx); err != nil || changed {
		t.Fatalf("RemoteChanged = %v %v after a sync", changed, err)
	}

	// a migration written on the server (dashboard automigrate) comes down, with no restart
	write(r.dir, "pb_migrations/1_created.js", "migrate(up)")
	if changed, _ := s.RemoteChanged(ctx); !changed {
		t.Fatal("RemoteChanged missed a server migration")
	}
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	expect(project, "pb_migrations/1_created.js", "migrate(up)")

	// hooks and pages pushed: no restart (hooks are watched)
	write(project, "pb_hooks/a.pb.js", "hook v2")
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if n := r.restarts.Load(); n != 0 {
		t.Fatalf("hooks push restarted (restarts = %d)", n)
	}

	// a pushed migration waits for the user's restart
	var logged []string
	s.opts.Log = func(l string) { logged = append(logged, l) }
	write(project, "pb_migrations/2_mine.js", "migrate(up2)")
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	s.opts.Log = func(string) {}
	expect(r.dir, "pb_migrations/2_mine.js", "migrate(up2)")
	if n := r.restarts.Load(); n != 0 {
		t.Fatalf("migration push: restarts = %d, want 0", n)
	}
	if !strings.Contains(strings.Join(logged, "\n"), "Migrations or packages changed") {
		t.Fatalf("no restart hint: %q", logged)
	}
	if _, err := c.Restart(ctx); err != nil || r.restarts.Load() != 1 {
		t.Fatalf("restart: %v (restarts %d)", err, r.restarts.Load())
	}

	// deletes go both ways
	os.Remove(filepath.Join(project, "pb_pages/index.ejs"))
	os.Remove(filepath.Join(r.dir, "pb_public/server.html"))
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	expect(r.dir, "pb_pages/index.ejs", "<missing>")
	expect(project, "pb_public/server.html", "<missing>")

	// changed on both sides: ours wins, theirs is saved aside
	write(project, "pb_hooks/a.pb.js", "hook mine")
	write(r.dir, "pb_hooks/a.pb.js", "hook theirs")
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	expect(r.dir, "pb_hooks/a.pb.js", "hook mine")
	expect(project, ".pocketone/conflicts/pb_hooks/a.pb.js", "hook theirs")

	// ignored files stay where they are
	write(r.dir, "pb_public/generated/out.txt", "generated")
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	expect(project, "pb_public/generated/out.txt", "<missing>")

	// a new Syncer picks up the saved state: no first-run question, nothing to do
	s2, err := NewSyncer(c, SyncOptions{CollectOptions: CollectOptions{Dir: project}})
	if err != nil {
		t.Fatal(err)
	}
	if s2.firstRun {
		t.Fatal("state wasn't saved")
	}
	if err := s2.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if n := r.restarts.Load(); n != 1 {
		t.Fatalf("restarts = %d at the end, want 1", n)
	}
}

func TestParsePartial(t *testing.T) {
	if p, err := parsePartial(map[string]any{"files": map[string]any{}}); p != nil || err != nil {
		t.Fatalf("full deploy: %v %v", p, err)
	}
	for _, bad := range []string{"pb_data/data.db", "node_modules/x/index.js", "../etc/passwd", "README.md"} {
		if _, err := parsePartial(map[string]any{"partial": true, "delete": []any{bad}}); err == nil {
			t.Errorf("delete %q accepted", bad)
		}
	}
	p, err := parsePartial(map[string]any{"partial": true, "delete": []any{"pb_hooks/a.pb.js"}})
	if err != nil || len(p.Delete) != 1 {
		t.Fatalf("%v %v", p, err)
	}
}
