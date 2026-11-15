package pocker

import (
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/dop251/goja"
	"github.com/pockethost/pocketone/ext/extkit"
)

// hostedExtension runs a PocketOne extension ([extkit.Extension], e.g. bulkmail.New(...)) on every instance: put the
// extension itself in [Server.Extensions], no pocker-specific wrapper needed.
//
// Settings: the instance's `extensions.<name>` over the defaults the extension was built with (process-wide settings are
// the extension's own, set in Go). `enabled: false` turns it off for that instance: Register and BindJS are never
// called.
type hostedExtension struct {
	p extkit.Extension
}

func hostExtension(p extkit.Extension) *hostedExtension { return &hostedExtension{p: p} }

func (h *hostedExtension) config(inst Instance) (extkit.Config, error) {
	own, err := extkit.Layer(inst.ExtensionConfig(h.p.Name()))
	if err != nil {
		return nil, fmt.Errorf("instance %s: extensions.%s: %w", inst.ID, h.p.Name(), err)
	}
	return extkit.Config{own}, nil
}

// InitApp implements [AppIniter].
func (h *hostedExtension) InitApp(inst Instance, app core.App) error {
	cfg, err := h.config(inst)
	if err != nil {
		return err
	}
	if !cfg.Enabled() {
		return nil
	}
	extkit.SetAppID(app, inst.ID)
	if err := h.p.Register(app, cfg); err != nil {
		return fmt.Errorf("extension %s: instance %s: %w", h.p.Name(), inst.ID, err)
	}
	return nil
}

// InitJS implements [JSIniter].
func (h *hostedExtension) InitJS(inst Instance, app core.App, vm *goja.Runtime) {
	b, ok := h.p.(extkit.JSBinder)
	if !ok {
		return
	}
	if cfg, err := h.config(inst); err == nil && cfg.Enabled() {
		b.BindJS(app, vm)
	}
}

// InitExtensions returns a ManagerOptions.Init that registers PocketOne extensions on each instance the way [Server] does,
// with each instance's `extensions.<name>` settings (there's no config file, so no process layer). For a Manager built
// by hand, e.g. in tests.
func InitExtensions(extensions ...extkit.Extension) func(Instance, core.App) error {
	hosted := make([]*hostedExtension, len(extensions))
	for i, p := range extensions {
		hosted[i] = &hostedExtension{p: p}
	}
	return func(inst Instance, app core.App) error {
		for _, h := range hosted {
			if err := h.InitApp(inst, app); err != nil {
				return err
			}
		}
		return nil
	}
}
