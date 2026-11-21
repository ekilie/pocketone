package deploy

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"pocketone.dev/ext/extkit"
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

// Migrations implements extkit.Migrator. 0001 renamed the deploy history collection _deploys (its name before extension
// tables were `_<extension>_<table>`) to _deploy_deploys; 0002 drops it, since deploys are no longer recorded.
func (p *Extension) Migrations() []extkit.Migration {
	return []extkit.Migration{
		extkit.RenameCollection("0001_rename_deploys", deploysCollectionId, "_deploys", "_deploy_deploys"),
		{Name: "0002_drop_deploys", Up: dropDeploys},
	}
}

// deploysCollectionId is the id the deploy history collection had under both names.
const deploysCollectionId = "pbc_deploys"

// dropDeploys deletes the deploy history collection and its rows, if it exists.
func dropDeploys(tx core.App) error {
	c, err := tx.FindCollectionByNameOrId(deploysCollectionId)
	if err != nil {
		return nil
	}
	// PocketBase refuses to delete a system collection: clear the flag first
	if c.System {
		c.System = false
		if err := tx.SaveNoValidate(c); err != nil {
			return err
		}
	}
	return tx.Delete(c)
}

// Register implements [extkit.Extension].
func (p *Extension) Register(app core.App, _ extkit.Config) error {
	_, err := Register(app, p.Config)
	return err
}

// Contracts implements [extkit.Declarer].
func (p *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{
		Implements: extkit.IDs("deploy/v1", "instance.fs/v1"),
		Defaults:   extkit.IDs("deploy/v1", "instance.fs/v1"), // a host provides them for its instances instead
		Requires:   extkit.IDs("remote/v1", "instance.fs/v1"), // restart and logs; the instance's folder
		WorksWith:  extkit.IDs("api-tokens/v1"),
	}
}
