package sandbox

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// A sandboxed app registers a superuser UI extension the documented way (pockethost.io/docs/admin-extensions): its
// own OnServe fires once when the app starts serving (Pocker's buildHandler), so hook files may bind it. Before, the
// sandbox hid $app.onServe and the file failed to load with "not a function".
func TestServeHookAndUIExtension(t *testing.T) {
	dir := t.TempDir()
	ext := filepath.Join(dir, "pb_admin_ext", "my-plugin")
	if err := os.MkdirAll(ext, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ext, "main.js"), []byte("// hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, run := sandboxed(t, &Sandbox{Dir: dir})
	if _, err := run(`
		$app.onServe().bindFunc((e) => {
			e.uiExtensions.push({ name: "my-plugin", fs: $os.dirFS(__hooks + "/../pb_admin_ext/my-plugin") })
			return e.next()
		})
	`); err != nil {
		t.Fatal(err)
	}
	ev := &core.ServeEvent{App: app}
	if err := app.OnServe().Trigger(ev); err != nil {
		t.Fatal(err)
	}
	if len(ev.UIExtensions) != 1 || ev.UIExtensions[0].Name != "my-plugin" {
		t.Fatalf("ui extensions: %+v", ev.UIExtensions)
	}
	if b, err := fs.ReadFile(ev.UIExtensions[0].FS, "main.js"); err != nil || string(b) != "// hello" {
		t.Fatalf("main.js: %q %v", b, err)
	}
	// still confined: the serve event's server and listener stay hidden, other hooks can't be bound at run time,
	// and the per-app lifecycle hooks are reached through their onX functions
	v, err := run(`[typeof $app.onServe().bindFunc, typeof $app.onRecordCreate().bindFunc, typeof onBootstrap,
		typeof onTerminate, typeof cronAdd].join("|")`)
	if err != nil || toString(v) != "function|undefined|function|function|function" {
		t.Fatalf("binds: %v %v", v, err)
	}
}
