package pages

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's key: its pocketone.yaml switch and an instance's `extensions.<name>`.
const Name = "pages"

// Extension is PocketPages as a PocketOne extension, a member of the standard set (ext/standard). Config holds the defaults
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
