package fsv1

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestIsReachable(t *testing.T) {
	for _, p := range []string{"pb_hooks/main.pb.js", "pb_public", "package.json", "bun.lock", "patches/x.patch",
		"node_modules/a/index.js", "pb_hooks/node_modules/a.js"} {
		if !IsReachable(p) {
			t.Errorf("%s should be reachable", p)
		}
	}
	for _, p := range []string{"pb_data", "pb_data/data.db", "pb_hooks/.git/config", ".gitignore", "README.md",
		"../etc/passwd", "pb_hooks/../pb_data/data.db", "/pb_hooks/x.js", ""} {
		if IsReachable(p) {
			t.Errorf("%s shouldn't be reachable", p)
		}
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "pb_public"), 0o755)
	if rel, p, err := Resolve(dir, "/pb_hooks/main.pb.js"); err != nil || rel != "pb_hooks/main.pb.js" || p != filepath.Join(dir, "pb_hooks", "main.pb.js") {
		t.Fatalf("%q %q %v", rel, p, err)
	}
	if _, p, err := Resolve(dir, ""); err != nil || p != dir {
		t.Fatalf("root: %q %v", p, err)
	}
	var pe *PathError
	if _, _, err := Resolve(dir, "pb_data/data.db"); !errors.As(err, &pe) {
		t.Fatalf("pb_data: %v", err)
	}
	_ = os.Symlink(t.TempDir(), filepath.Join(dir, "pb_public", "out"))
	if _, _, err := Resolve(dir, "pb_public/out/x"); !errors.As(err, &pe) || pe.Reason != "crosses a symbolic link" {
		t.Fatalf("symlink: %v", err)
	}
}

func TestLocalRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	running, stops, starts := true, 0, 0
	l := &Local{
		Folder:    func() string { return dir },
		IsRunning: func() bool { return running },
		Stopped: func(ctx context.Context, fn func(context.Context) error) error {
			stops++
			return fn(ctx)
		},
		Start: func(context.Context) error { starts++; return nil },
	}
	applied := 0
	apply := func(context.Context) error { applied++; return nil }
	if startErr, err := l.Restart(ctx, apply); startErr != nil || err != nil || stops != 1 || starts != 1 || applied != 1 {
		t.Fatalf("running: %v %v %d %d %d", startErr, err, stops, starts, applied)
	}
	running = false
	if _, err := l.Restart(ctx, apply); err != nil || stops != 1 || starts != 1 || applied != 2 {
		t.Fatalf("stopped: %v %d %d %d", err, stops, starts, applied)
	}
	if d, err := l.StateDir(ctx, "deploy"); err != nil || d != filepath.Join(dir, ".pb_deploy") {
		t.Fatalf("state: %q %v", d, err)
	}
	if _, err := l.StateDir(ctx, "../x"); err == nil {
		t.Fatal("bad state name accepted")
	}
	if _, err := (&Local{Folder: func() string { return "" }}).Dir(ctx); !errors.Is(err, ErrNoFolder) {
		t.Fatal(err)
	}
}
