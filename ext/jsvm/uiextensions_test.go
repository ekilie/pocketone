package jsvm

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// A pb_hooks file registers a superuser UI extension the way PocketHost documents it (pockethost.io/docs/admin-extensions):
// e.uiExtensions.push({ name, fs: $os.dirFS(dir) }) in $app.onServe(). The fs is a Go value inside a JS object, so the
// struct is filled field by field; through JSON it became a string and the app failed to start.
func TestUIExtensionFromHooks(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte("// hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	vm := testVM(t)
	if err := BindApp(vm, app); err != nil {
		t.Fatal(err)
	}
	if err := vm.Do(func() { vm.setGo("extDir", dir) }); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(`
		$app.onServe().bindFunc((e) => {
			e.uiExtensions.push({ name: "my-plugin", fs: $os.dirFS(extDir) })
			return e.next()
		})
	`); err != nil {
		t.Fatal(err)
	}

	ev := &core.ServeEvent{App: app}
	if err := app.OnServe().Trigger(ev); err != nil {
		t.Fatal(err)
	}
	if len(ev.UIExtensions) != 1 || ev.UIExtensions[0].Name != "my-plugin" || ev.UIExtensions[0].FS == nil {
		t.Fatalf("ui extensions: %+v", ev.UIExtensions)
	}
	b, err := fs.ReadFile(ev.UIExtensions[0].FS, "main.js")
	if err != nil || string(b) != "// hello" {
		t.Fatalf("main.js: %q %v", b, err)
	}
}

// A struct from a plain object with no Go values still goes through JSON (custom unmarshalers, json tags).
func TestStructFromPlainObjectStillJSON(t *testing.T) {
	vm := testVM(t)
	var got core.UIExtension
	if err := vm.Do(func() { vm.setGo("take", func(x core.UIExtension) { got = x }) }); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(`take({ name: "plain" })`); err != nil {
		t.Fatal(err)
	}
	if got.Name != "plain" || got.FS != nil {
		t.Fatalf("got %+v", got)
	}
}
