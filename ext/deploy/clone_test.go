package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	remoteclient "pocketone.dev/ext/contracts/remote/v1/remoteclient"
)

func TestClone(t *testing.T) {
	t.Setenv("P1_CONFIG_DIR", t.TempDir())
	t.Setenv("P1_TOKEN", "")
	r := newP1Remote(t)
	ctx := context.Background()
	token := r.token(t, "_superusers", "test@example.com", ScopeDeploy, "remote", "remote:read")
	write := func(root, rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(r.dir, "pb_pages/index.ejs", "page")
	write(r.dir, "pb_hooks/main.pb.js", "hook")
	write(r.dir, "package.json", `{"dependencies":{}}`)
	write(r.dir, "bun.lock", "lock")
	write(r.dir, "patches/x.patch", "patch")
	write(r.dir, "pb_data/data.db", "never")
	login := func(ctx context.Context, c *remoteclient.Client) error {
		_, err := remoteclient.Login(ctx, c, remoteclient.LoginOptions{Token: token})
		return err
	}

	// a clone inside another project is a project of its own
	outer := t.TempDir()
	write(outer, ".pocketbase/remotes.json", `{"default":"other","remotes":{"other":{"url":"https://other.example.com"}}}`)
	dir, n, err := Clone(ctx, CloneOptions{URL: r.url, Dir: filepath.Join(outer, "app"), Login: login})
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("pulled %d files, want 5", n)
	}
	for _, rel := range []string{"pb_pages/index.ejs", "pb_hooks/main.pb.js", "package.json", "bun.lock", "patches/x.patch"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("%s not cloned: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "pb_data")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("pb_data came down: %v", err)
	}
	p, err := remoteclient.LoadProject(dir)
	if err != nil || p.Root != dir || p.Default != remoteclient.DefaultName || p.Remotes[remoteclient.DefaultName].URL != r.url {
		t.Fatalf("project = %+v, %v", p, err)
	}

	// the clone recorded its base: deploying straight back has nothing to stop on
	rc, err := remoteclient.Open(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(ctx, rc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Deploy(ctx, c, DeployOptions{CollectOptions: CollectOptions{Dir: dir}}); err != nil {
		t.Fatalf("deploy after clone: %v", err)
	}

	// a folder that isn't empty is refused and left alone
	if _, _, err := Clone(ctx, CloneOptions{URL: r.url, Dir: dir, Login: login}); err == nil || !strings.Contains(err.Error(), "isn't empty") {
		t.Fatalf("clone into a full folder: %v", err)
	}

	// a failed sign-in removes the folder clone made
	failed := filepath.Join(t.TempDir(), "nope")
	bad := func(ctx context.Context, c *remoteclient.Client) error {
		_, err := remoteclient.Login(ctx, c, remoteclient.LoginOptions{Token: "pbt_wrong"})
		return err
	}
	if _, _, err := Clone(ctx, CloneOptions{URL: r.url, Dir: failed, Login: bad}); err == nil {
		t.Fatal("clone with a bad token succeeded")
	}
	if _, err := os.Stat(failed); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("folder left after a failed sign-in: %v", err)
	}
}

func TestCloneDirName(t *testing.T) {
	for in, want := range map[string]string{
		"https://pocketcon.dev":        "pocketcon",
		"https://myapp.pockethost.io/": "myapp",
		"http://localhost:8090":        "app",
		"http://127.0.0.1:8090":        "app",
		"https://[::1]:8443":           "app",
	} {
		if got := CloneDirName(in); got != want {
			t.Errorf("CloneDirName(%q) = %q, want %q", in, got, want)
		}
	}
}
