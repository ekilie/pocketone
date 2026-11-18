package pocker

import (
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// hostedExtension runs a PocketOne extension ([extkit.Extension], e.g. bulkmail.New(...)) on every instance: put the
// extension itself in [Server.Extensions], no pocker-specific wrapper needed.
//
// Settings: the instance's `extensions.<name>` over its section of pocker's isolate_config, over the defaults the
// extension was built with. `enabled: false` turns it off for that instance: Register and BindJS are never
// called.
type hostedExtension struct {
	p    extkit.Extension
	base map[string]any // its section of pocker's isolate_config, under each instance's own
}

func hostExtension(p extkit.Extension, base map[string]any) *hostedExtension {
	return &hostedExtension{p: p, base: base}
}

func (h *hostedExtension) config(inst Instance) (extkit.Config, error) {
	own, err := extkit.Layer(inst.ExtensionConfig(h.p.Name()))
	if err != nil {
		return nil, fmt.Errorf("instance %s: extensions.%s: %w", inst.ID, h.p.Name(), err)
	}
	return extkit.Config{h.base, own}, nil
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
func (h *hostedExtension) InitJS(inst Instance, app core.App, rt js.Runtime) {
	b, ok := h.p.(extkit.JSBinder)
	if !ok {
		return
	}
	if cfg, err := h.config(inst); err == nil && cfg.Enabled() {
		b.BindJS(app, rt)
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
