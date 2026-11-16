package mcp

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// Name is the extension's key: its pocketone.yaml switch and an instance's `extensions.<name>`.
const Name = "mcp"

// Extension is the MCP endpoint as a PocketOne extension, a member of the standard set (ext/standard). Config holds the defaults
// for every app; an instance's `extensions.mcp.enabled: false` turns it off there.
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

// BindJS implements [extkit.JSBinder].
func (p *Extension) BindJS(app core.App, rt js.Runtime) { BindJS(app, rt) }

// Contracts implements [extkit.Declarer].
func (p *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{
		Implements: extkit.IDs("mcp/v1"),
		Defaults:   extkit.IDs("mcp/v1"), // another MCP server can replace it; the contract tools stay
		WorksWith:  extkit.IDs("tokens/v1", "oauth/v1"),
	}
}
