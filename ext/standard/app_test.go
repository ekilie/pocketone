package standard_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pocketone.dev/ext/config"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/standard"
)

// The app section reaches every field a single app is built from, with paths relative to the config file.
func TestAppFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pocketone.yaml")
	yaml := `app:
  dir: ./data
  dev: true
  encryption_env: MY_KEY
  query_timeout: 7s
  jsvm: goja
  hooks_pool: 3
  hooks_dir: ./hooks
  migrations_dir: /abs/migrations
  js_limits:
    memory: 1GiB
    task_time: 2m
    task_wall: 10m
    await_time: 1h
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.Options{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { extkit.SetSectionSource(nil) })

	a, err := standard.AppFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pb := a.PocketBase
	if pb.DefaultDataDir != filepath.Join(dir, "data") || !pb.DefaultDev || pb.DefaultEncryptionEnv != "MY_KEY" ||
		pb.DefaultQueryTimeout != 7*time.Second {
		t.Errorf("PocketBase: %+v", pb)
	}
	js := a.JS
	if js.HooksDir != filepath.Join(dir, "hooks") || js.MigrationsDir != "/abs/migrations" || js.HooksPoolSize != 3 {
		t.Errorf("JS: %+v", js)
	}
	if js.VM.MemoryLimit != 1<<30 || js.VM.TaskBudget != 2*time.Minute || js.VM.TaskWall != 10*time.Minute ||
		js.VM.AwaitLimit != time.Hour {
		t.Errorf("VM: %+v", js.VM)
	}
	if a.JSEngine != standard.JSEngineGoja {
		t.Errorf("JSEngine: %q", a.JSEngine)
	}
}

// With no config file every key keeps its default: quickjs, a pool of 15, ext/jsvm's own limits.
func TestAppFromConfigDefaults(t *testing.T) {
	extkit.SetSectionSource(nil)
	a, err := standard.AppFromConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.JSEngine != standard.JSEngineQuickJS || a.JS.HooksPoolSize != 15 || a.JS.VM.MemoryLimit != 0 ||
		a.PocketBase.DefaultDataDir != "" {
		t.Errorf("defaults: %+v", a)
	}
}
