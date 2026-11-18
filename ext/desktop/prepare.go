package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

// Prepare writes the build image's module into dir: a main package against source, its go.mod, and every dependency
// vendored (vendor/ holds every platform's files), so builds in the image need neither the source nor the network.
// A build then only rewrites main.go and app/ (TemplateEnv).
func Prepare(ctx context.Context, source, dir string) error {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return errors.New("desktop prepare needs Go")
	}
	if !isSource(source) {
		return errors.New(source + " isn't a PocketOne source tree (no ext/desktop/runtime)")
	}
	if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
		return err
	}
	// a placeholder app, so the main package compiles while vendoring
	if err := os.WriteFile(filepath.Join(dir, "app", ".keep"), nil, 0o644); err != nil {
		return err
	}
	if err := writeMain(dir, "App", "app.pocketone.app", Config{}, nil); err != nil {
		return err
	}
	if err := writeGoMod(dir, source); err != nil {
		return err
	}
	for _, args := range [][]string{{"mod", "tidy"}, {"mod", "vendor"}} {
		if err := goCmd(ctx, goBin, dir, nil, args...); err != nil {
			return err
		}
	}
	return nil
}
