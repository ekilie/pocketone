// Package pockerext serves pb_pages for every pocker instance. It's its own package so single-app builds of
// ext/pages (like the standard PocketOne binary) don't link pocker.
package pockerext

import (
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/multisite-provider-pocker"
	"github.com/pockethost/pocketone/ext/pages"
)

// Name is the extension's name under pocker: an instance's `extensions.pages` block.
const Name = "pages"

// Extension serves each pocker instance's pb_pages (<instance dir>/pb_pages). Every start of an instance's app gets
// its own route table and caches, so an instance that sleeps (its app stops) drops them, and one instance never sees
// another's files. Page code runs in the instance's own sandboxed JS runtimes, with the instance's limits.
//
//	srv := &pocker.Server{Config: cfg, Extensions: []pocker.Extension{pockerext.New(pages.Config{})}}
//
// An instance turns pages off with `extensions: { pages: { enabled: false } }`.
type Extension struct{ cfg pages.Config }

// New returns the pocker extension. cfg applies to every instance; Dir is ignored (each instance uses its own folder).
func New(cfg pages.Config) *Extension { return &Extension{cfg: cfg} }

// Name implements [pocker.Extension].
func (p *Extension) Name() string { return Name }

type instanceConfig struct {
	Enabled *bool `yaml:"enabled"`
}

// InitApp implements [pocker.AppIniter]: it registers pages on the instance's app before it bootstraps, so the
// router middleware joins the app's OnServe as it does for a single app.
func (p *Extension) InitApp(inst pocker.Instance, app core.App) error {
	var ic instanceConfig
	if err := pocker.DecodeExtensionConfig(inst.ExtensionConfig(Name), &ic); err != nil {
		return fmt.Errorf("pages: extensions.pages: %w", err)
	}
	if ic.Enabled != nil && !*ic.Enabled {
		return nil
	}
	cfg := p.cfg
	cfg.Dir = "" // <instance dir>/pb_pages, next to pb_data
	_, err := pages.Register(app, cfg)
	return err
}
