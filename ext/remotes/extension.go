package remotes

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"pocketone.dev/ext/extkit"
)

// Name is the extension's key: its pocketone.yaml switch and an instance's `extensions.<name>`.
const Name = "remotes"

// Extension is remotes (whoami) as a PocketOne extension, a member of the standard set (ext/standard). Config holds the defaults
// for every app; an instance's `extensions.remotes.enabled: false` turns it off there.
type Extension struct{ Config Config }

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension with cfg as every app's defaults.
func New(cfg Config) *Extension { return &Extension{Config: cfg} }

// Name implements [extkit.Extension].
func (p *Extension) Name() string { return Name }

// Register implements [extkit.Extension].
func (p *Extension) Register(app core.App, _ extkit.Config) error {
	return Register(app, p.Config)
}

// Contracts implements [extkit.Declarer].
func (p *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{
		Implements: extkit.IDs("remote/v1"),
		Defaults:   extkit.IDs("remote/v1"), // a multisite host provides it for its instances instead
		WorksWith:  extkit.IDs("api-tokens/v1"),
	}
}
