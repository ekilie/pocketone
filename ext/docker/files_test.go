package docker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
	"pocketone.dev/ext/multisite"
)

// TestFiles: instance.fs/v1 for the host's instances, without a Docker daemon: known instances only, their folder
// under DataRoot, one FS per instance, and a restart that holds the instance while the change applies.
func TestFiles(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	rt := &Runtime{cfg: Config{DataRoot: data}, ctrs: map[string]*ctr{}}
	dir := multisite.NewDirectory(slog.New(slog.NewTextHandler(io.Discard, nil)))
	dir.Load([]multisite.Instance{{ID: "abc", Hosts: []string{"abc.example.com"}}})
	files := rt.Files(dir)

	if _, err := files.Instance(ctx, "nope"); !errors.Is(err, fsv1.ErrNoInstance) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := files.Instance(ctx, fsv1.Self); !errors.Is(err, fsv1.ErrNoInstance) {
		t.Fatalf("self: %v", err)
	}
	f, err := files.Instance(ctx, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := files.Instance(ctx, "abc"); again != f {
		t.Fatal("a second FS for the same instance (its lock wouldn't be shared)")
	}
	if d, _ := f.Dir(ctx); d != filepath.Join(data, "abc") {
		t.Fatalf("dir: %s", d)
	}

	// not running: the change applies, nothing is held
	if _, err := f.Restart(ctx, func(context.Context) error {
		if rt.held["abc"] != nil {
			t.Error("held while not running")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// running: the instance is held while the change applies
	rt.get("abc").running.Store(true)
	held := false
	if _, err := f.Restart(ctx, func(context.Context) error {
		rt.mu.Lock()
		_, held = rt.held["abc"]
		rt.mu.Unlock()
		return nil
	}); err != nil || !held {
		t.Fatalf("running: held=%v err=%v", held, err)
	}
}
