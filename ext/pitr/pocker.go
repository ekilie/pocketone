package pitr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/pocker"
)

// The extension under a multitenant pocker server:
//
//	srv.Extensions = append(srv.Extensions, pitr.New(pitr.Config{}))
//
// Process settings come from pocketone.yaml's `pitr` section (fields set in Go win), per-instance settings
// from the instance's `extensions: {pitr: {enabled, retention}}` (pocker hosts it as an [extkit.Extension]).
var (
	_ pocker.Extension  = (*Extension)(nil)
	_ pocker.Configurer = (*Extension)(nil)
	_ extkit.Extension  = (*Extension)(nil)
	_ pocker.Starter    = (*Extension)(nil)
)

// Configure reads pocketone.yaml's `pitr` section. Fields already set in Go are kept; the section fills the rest.
func (p *Extension) Configure(pocker.Config) error {
	c, found := Section.Get()
	if !found {
		return nil
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
	return nil
}

// settings is the shape of an instance's `extensions.pitr`. Only `enabled` and `retention` are read per app.
type settings struct {
	Enabled   *bool            `json:"enabled"`
	Retention *extkit.Duration `json:"retention"`
}

// Register implements [extkit.Extension]: sets up app before it bootstraps. Where its history goes is decided per
// app: its own PITR S3 setting, else the server's storage (`litestream` kind), else its pb_data/litestream/.
func (p *Extension) Register(app core.App, layers extkit.Config) error {
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

// Start waits for the server to stop, then for every app to flush and detach (the manager stops them), and closes
// the Litestream stores.
func (p *Extension) Start(ctx context.Context, _ *pocker.Manager) {
	<-ctx.Done()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		n := len(p.apps)
		p.mu.Unlock()
		if n == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.Close(cctx); err != nil && !errors.Is(err, context.Canceled) {
		p.log.Warn("pitr: close", "error", err)
	}
}

// Section is the key of the `pitr` section (pocketone.yaml): point-in-time restore defaults for a single app (the
// pocketone binary reads it for its flag defaults) and for every instance on a pocker host.
var Section = extkit.NewSection(Name, Config{}, "Point-in-time restore: on by default, retention, snapshot interval. The --pitr and --pitrRetention flags win.")
