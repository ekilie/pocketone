package deploy

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's key: its pocketone.yaml switch and an instance's `extensions.<name>`.
const Name = "deploy"

// Extension is the deploy API as a PocketOne extension, a member of the standard set (ext/standard). Config holds the defaults
// for every app; an instance's `extensions.deploy.enabled: false` turns it off there.
type Extension struct{ Config Config }

var _ extkit.Extension = (*Extension)(nil)

// NewExtension returns the extension with cfg as every app's defaults (New builds the deploy API itself).
func NewExtension(cfg Config) *Extension { return &Extension{Config: cfg} }

// Name implements [extkit.Extension].
func (p *Extension) Name() string { return Name }

// Migrations implements extkit.Migrator. 0001 renames _deploys (its name before extension tables were
// `_<extension>_<table>`) to _deploy_deploys; the id stays.
func (p *Extension) Migrations() []extkit.Migration {
	return []extkit.Migration{extkit.RenameCollection("0001_rename_deploys", DeploysCollectionId, "_deploys", DeploysCollection)}
}

// Register implements [extkit.Extension].
func (p *Extension) Register(app core.App, _ extkit.Config) error {
	_, err := Register(app, p.Config)
	return err
}

// Contracts implements [extkit.Declarer].
func (p *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{
		Implements: extkit.IDs("deploy/v1"),
		Defaults:   extkit.IDs("deploy/v1"), // a multisite host provides it for its instances instead
		Requires:   extkit.IDs("remote/v1"), // restart and logs
		WorksWith:  extkit.IDs("tokens/v1"),
	}
}
