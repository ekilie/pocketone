package jsvm

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// registerBootstrapped registers the engine on a test app and bootstraps it (types are written after bootstrap).
func registerBootstrapped(t *testing.T, cfg Config) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	if err := Register(app, cfg); err != nil {
		t.Fatal(err)
	}
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
}

// With no TypesDir, types.d.ts and tsconfig.json go in the project root (the folder holding pb_hooks), not pb_data
// and not pb_hooks.
func TestTypesDirDefaultsToProjectRoot(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"pb_hooks/.keep": ""})
	registerBootstrapped(t, Config{HooksDir: filepath.Join(root, "pb_hooks"), MigrationsDir: filepath.Join(root, "pb_migrations")})

	for _, name := range []string{"types.d.ts", "tsconfig.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s not in the project root: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(root, "pb_hooks", name)); err == nil {
			t.Errorf("%s written into pb_hooks", name)
		}
	}

	var cfg struct {
		CompilerOptions map[string]any `json:"compilerOptions"`
		Include         []string       `json:"include"`
	}
	raw, _ := os.ReadFile(filepath.Join(root, "tsconfig.json"))
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("tsconfig.json isn't JSON: %v\n%s", err, raw)
	}
	want := []string{"types.d.ts", "pb_hooks/**/*", "pb_migrations/**/*", "pb_pages/**/*"}
	if !slices.Equal(cfg.Include, want) {
		t.Errorf("include = %q, want %q", cfg.Include, want)
	}
	if cfg.CompilerOptions["skipLibCheck"] != true {
		t.Errorf("skipLibCheck not set: %v", cfg.CompilerOptions)
	}
}

// An existing tsconfig.json is the developer's: never overwritten.
func TestTSConfigWrittenOnce(t *testing.T) {
	root := t.TempDir()
	mine := `{"include": ["src/**/*"]}`
	writeFiles(t, root, map[string]string{"tsconfig.json": mine, "pb_hooks/.keep": ""})
	registerBootstrapped(t, Config{HooksDir: filepath.Join(root, "pb_hooks"), MigrationsDir: filepath.Join(root, "pb_migrations")})

	if got, _ := os.ReadFile(filepath.Join(root, "tsconfig.json")); string(got) != mine {
		t.Fatalf("tsconfig.json overwritten:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "types.d.ts")); err != nil {
		t.Fatalf("types.d.ts not written: %v", err)
	}
}

// Hooks and migrations folders elsewhere are included where they are.
func TestTSConfigFollowsTheFolders(t *testing.T) {
	root := t.TempDir()
	p := &plugin{config: Config{TypesDir: root, HooksDir: filepath.Join(root, "src", "hooks"), MigrationsDir: filepath.Join(root, "db")}}
	var cfg struct{ Include []string }
	if err := json.Unmarshal(p.tsconfigContent(), &cfg); err != nil {
		t.Fatal(err)
	}
	want := []string{"types.d.ts", "src/hooks/**/*", "db/**/*", "pb_pages/**/*"}
	if !slices.Equal(cfg.Include, want) {
		t.Fatalf("include = %q, want %q", cfg.Include, want)
	}
}

// An empty hook file gets a reference to types.d.ts in the project root.
func TestEmptyHookReferencesProjectTypes(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"pb_hooks/main.pb.js": ""})
	registerBootstrapped(t, Config{HooksDir: filepath.Join(root, "pb_hooks"), MigrationsDir: filepath.Join(root, "pb_migrations")})

	for file, want := range map[string]string{
		"pb_hooks/main.pb.js": `/// <reference path="../types.d.ts" />`,
	} {
		got, _ := os.ReadFile(filepath.Join(root, file))
		if string(got) != want+"\n\n" {
			t.Errorf("%s = %q, want %q", file, got, want)
		}
	}
}

// Hooks embedded in the binary have nothing on disk to type: no types.d.ts or tsconfig.json unless TypesDir is set.
func TestEmbeddedHooksWriteNoTypes(t *testing.T) {
	root := t.TempDir()
	registerBootstrapped(t, Config{HooksDir: filepath.Join(root, "pb_hooks"), MigrationsDir: t.TempDir(), Extensions: []any{&testExt{client: &http.Client{}}}})
	for _, name := range []string{"types.d.ts", "tsconfig.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			t.Errorf("%s written for embedded hooks", name)
		}
	}
}
