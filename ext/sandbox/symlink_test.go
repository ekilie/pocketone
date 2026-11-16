package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"github.com/pockethost/pocketone/ext/extkit/js"
	"github.com/pockethost/pocketone/ext/jsvm"
)

// sandboxed registers jsvm with s on a test app (hooks in s.Dir/pb_hooks) and returns a function running JS on it.
func sandboxed(t *testing.T, s *Sandbox) (core.App, func(src string) (any, error)) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	exec, sleep := s.Limits()
	cfg := jsvm.Config{
		HooksDir:      filepath.Join(s.Dir, "pb_hooks"),
		MigrationsDir: filepath.Join(s.Dir, "pb_migrations"),
		TypesDir:      t.TempDir(),
		Extensions:    []any{s},
		VM:            jsvm.VMConfig{TaskWall: exec, MaxSleep: sleep},
	}
	if err := jsvm.Register(app, cfg); err != nil {
		t.Fatal(err)
	}
	return app, func(src string) (any, error) {
		var out any
		err := js.Run(app, func(rt js.Runtime) error {
			v, err := rt.Engine().(*jsvm.VM).RunString(src)
			out = v.Export()
			return err
		})
		return out, err
	}
}

// require() under a folder reached through a symlink (macOS temp folders: /var -> /private/var) still counts as
// inside; a real path outside the app folder is refused.
func TestRequireThroughSymlinkedDir(t *testing.T) {
	real := t.TempDir()
	if err := os.MkdirAll(filepath.Join(real, "app", "pb_hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "app", "pb_hooks", "lib.js"), []byte(`module.exports = { name: "own lib" }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "outside.js"), []byte(`module.exports = { name: "outside" }`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Join(real, "app"), link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	_, run := sandboxed(t, &Sandbox{Dir: link})
	for _, src := range []string{`require(__hooks + "/lib.js").name`, `require(__hooks + "/lib").name`, `require("./lib.js").name`} {
		v, err := run(src)
		if err != nil || v != "own lib" {
			t.Fatalf("%s: %v %v", src, v, err)
		}
	}
	if _, err := run(`require("` + filepath.Join(real, "outside.js") + `")`); err == nil {
		t.Fatal("required a file outside the app folder")
	}
}

// What a sandboxed app's JS can't reach.
func TestConfinement(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pb_hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "mine.txt"), []byte("mine"), 0o644)
	secret := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(secret, []byte("secret"), 0o644)
	t.Setenv("HOST_SECRET", "host")

	_, run := sandboxed(t, &Sandbox{Dir: dir, Env: map[string]string{"APP_KEY": "app"}})
	check := func(src, want string) {
		t.Helper()
		v, err := run(src)
		got := ""
		if err != nil {
			got = "err:" + err.Error()
		} else {
			got = toString(v)
		}
		if !strings.Contains(got, want) {
			t.Errorf("%s: got %q, want %q", src, got, want)
		}
	}
	check(`toString($os.readFile("mine.txt"))`, "mine")
	check(`toString($os.readFile("`+secret+`"))`, "err:")
	check(`toString($os.readFile("../`+filepath.Base(filepath.Dir(secret))+`/secret.txt"))`, "err:")
	check(`$os.exec("ls")`, "sandboxed")
	check(`$os.cmd("ls")`, "sandboxed")
	check(`$os.exit(1)`, "sandboxed")
	check(`$filesystem.local("/")`, "sandboxed")
	check(`$os.getenv("APP_KEY") + "|" + $os.getenv("HOST_SECRET")`, "app|")
	check(`process.env.APP_KEY + "|" + (process.env.HOST_SECRET === undefined)`, "app|true")
	check(`typeof $app.restart + "|" + typeof $app.bootstrap + "|" + typeof $app.onServe + "|" + typeof $app.findRecordById`, "undefined|undefined|undefined|function")
	check(`typeof $app.db().db`, "undefined")
	check(`$os.args.length`, "0")
}

func toString(v any) string { return fmt.Sprint(v) }
