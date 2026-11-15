package pocker

import (
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/dop251/goja"
	"github.com/pockethost/pocketone/ext/extkit"
)

type fakeExtension struct {
	got   map[string]int // instance id -> decoded daily
	bound map[string]bool
}

type fakeSettings struct {
	Daily int `json:"daily"`
}

func (f *fakeExtension) Name() string { return "fake" }

func (f *fakeExtension) Register(app core.App, cfg extkit.Config) error {
	s := fakeSettings{Daily: 1} // the extension's own default
	if err := cfg.Decode(&s); err != nil {
		return err
	}
	f.got[app.Settings().Meta.AppName] = s.Daily
	return nil
}

func (f *fakeExtension) BindJS(app core.App, _ *goja.Runtime) {
	f.bound[app.Settings().Meta.AppName] = true
}

func TestHostedExtension(t *testing.T) {
	f := &fakeExtension{got: map[string]int{}, bound: map[string]bool{}}
	h := hostExtension(f)

	run := func(inst Instance) {
		t.Helper()
		app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
		app.Settings().Meta.AppName = inst.ID
		if err := h.InitApp(inst, app); err != nil {
			t.Fatal(err)
		}
		h.InitJS(inst, app, goja.New())
	}

	run(Instance{ID: "plain"})                                                                     // the extension's own defaults
	run(Instance{ID: "own", Extensions: map[string]any{"fake": map[string]any{"daily": 7}}})       // instance wins
	run(Instance{ID: "typed", Extensions: map[string]any{"fake": &fakeSettings{Daily: 3}}})        // a host's typed value
	run(Instance{ID: "off", Extensions: map[string]any{"fake": map[string]any{"enabled": false}}}) // turned off

	want := map[string]int{"plain": 1, "own": 7, "typed": 3}
	for id, d := range want {
		if f.got[id] != d || !f.bound[id] {
			t.Fatalf("%s: daily %d bound %v, want %d", id, f.got[id], f.bound[id], d)
		}
	}
	if _, ok := f.got["off"]; ok || f.bound["off"] {
		t.Fatal("enabled: false still registered or bound")
	}
}

// Extensions read their settings at Register, so a change to an instance's extension settings restarts its app.
func TestExtensionSettingsChangeRestarts(t *testing.T) {
	a := Instance{ID: "a", Extensions: map[string]any{"fake": map[string]any{"daily": 7}}}
	b := a
	b.Extensions = map[string]any{"fake": map[string]any{"daily": 8}}
	if !sameRuntime(a, a) {
		t.Fatal("identical instances must share an app")
	}
	if sameRuntime(a, b) {
		t.Fatal("changed extension settings must restart the app")
	}
}
