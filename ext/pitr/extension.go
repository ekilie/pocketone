package pitr

import (
	"fmt"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"pocketone.dev/ext/extkit"
)

// Process settings come from pocketone.yaml's `pitr` section (fields set in Go win), read when the extension first
// registers; per-instance settings from the instance's `extensions: {pitr: {enabled, retention}}`. On a multisite host
// the stores close through extkit.Closer once every app has stopped.
var (
	_ extkit.Extension = (*Extension)(nil)
	_ extkit.Closer    = (*Extension)(nil)
)

// Contracts implements [extkit.Declarer]: pitr is restore/v1 at any moment in its window. Turned off for the whole
// process (--pitr=false, `pitr: {enabled: false}`) it declares nothing, so the backups default answers instead.
func (p *Extension) Contracts() extkit.Contracts {
	p.applySection()
	if p.cfg.Enabled != nil && !*p.cfg.Enabled {
		return extkit.Contracts{}
	}
	return extkit.Contracts{Implements: extkit.IDs("restore/v1"), WorksWith: extkit.IDs("settings/v1")}
}

// applySection reads pocketone.yaml's `pitr` section, once. Fields already set in Go are kept; the section fills the rest.
func (p *Extension) applySection() {
	p.sectionOnce.Do(func() {
		c, found := Section.Get()
		if !found {
			return
		}
		if p.cfg.Enabled == nil {
			p.cfg.Enabled = c.Enabled
		}
		if !p.retentionSet && c.Retention > 0 {
			p.cfg.Retention = c.Retention
		}
		if !p.intervalSet && c.SnapshotInterval > 0 {
			p.cfg.SnapshotInterval = c.SnapshotInterval
		}
	})
}

// settings is the shape of an instance's `extensions.pitr`. Only `enabled` and `retention` are read per app.
type settings struct {
	Enabled   *bool            `json:"enabled"`
	Retention *extkit.Duration `json:"retention"`
}

// Register implements [extkit.Extension]: sets up app before it bootstraps. Where its history goes is decided per
// app: its own PITR S3 setting, else the server's storage (`litestream` kind), else its pb_data/litestream/.
func (p *Extension) Register(app core.App, layers extkit.Config) error {
	p.applySection()
	var s settings
	if err := layers.Decode(&s); err != nil {
		return fmt.Errorf("pitr: %w", err)
	}
	ac := AppConfig{Enabled: s.Enabled}
	if s.Retention != nil {
		ac.Retention = time.Duration(*s.Retention)
	}
	return p.SetupApp(app, ac)
}

// Section is the key of the `pitr` section (pocketone.yaml): point-in-time restore defaults for a standalone app (the
// pocketone binary reads it for its flag defaults) and for every instance a multisite host runs as an isolate (Pocker).
var Section = extkit.NewSection(Name, Config{}, "Point-in-time restore: on by default, retention, snapshot interval. The --pitr and --pitrRetention flags win.")
