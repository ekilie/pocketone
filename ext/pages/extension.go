package pages

import (
	_ "embed"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"pocketone.dev/ext/extkit"
)

// Name is the extension's key: its pocketone.yaml switch and an instance's `extensions.<name>`.
const Name = "pages"

// Extension is Pages as a PocketOne extension, a member of the standard set (ext/standard). Config holds the defaults
// for every app; an instance's `extensions.pages.enabled: false` turns it off there.
type Extension struct{ Config Config }

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension with cfg as every app's defaults.
func New(cfg Config) *Extension { return &Extension{Config: cfg} }

// Name implements [extkit.Extension].
func (p *Extension) Name() string { return Name }

// Register implements [extkit.Extension].
func (p *Extension) Register(app core.App, _ extkit.Config) error {
	_, err := Register(app, p.Config)
	return err
}

// Contracts implements [extkit.Declarer].
func (p *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{WorksWith: extkit.IDs("cache/v1")}
}

//go:embed pages.d.ts
var jsTypes string

// TypeScript implements the JS engine's Typer (extkit/js): the types of page code (pages.Context, pages.Handlers,
// pages.Middleware, pages.Config, pages.Props, JSX), added to the app's types.d.ts. Page code gets no globals from
// them; files use the types by name.
func (p *Extension) TypeScript() string { return jsTypes }
