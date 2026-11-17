// Package vacuum compacts an app's SQLite databases on a schedule its admins set (SQLite's VACUUM), so space freed by
// deleted records goes back to the disk. A database is only compacted when enough of it is free space. It provides
// vacuum/v1 (the status) and a settings/v1 section (on/off, how often); pocketone.yaml's `vacuum` section sets the
// defaults. On a multisite host that provides vacuum/v1 for its instances, it steps aside: the host compacts them on
// its own schedule and the app's admins have no setting.
package vacuum

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"

	settingsv1 "github.com/pockethost/pocketone/ext/contracts/settings/v1"
	vacuumv1 "github.com/pockethost/pocketone/ext/contracts/vacuum/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's key.
const Name = "vacuum"

// Config is the `vacuum` section of pocketone.yaml: the defaults until the app's admins change them in its settings.
//
//	vacuum:
//	  enabled: true
//	  every: 168h
type Config struct {
	Enabled bool   `yaml:"enabled" doc:"Compact the databases on a schedule, until the app's admins change it in Settings."`
	Every   string `yaml:"every" doc:"How often, e.g. 24h or 168h (weekly)."`
}

// Section is the `vacuum` section.
var Section = extkit.NewSection(Name, Config{Every: "168h"}, "Defaults for compacting the app's SQLite databases (VACUUM): on or off, and how often. The app's admins change them in Settings.")

// A database is compacted only when at least this share of it, or this many bytes, is free space.
const (
	MinFreeRatio = 0.10
	MinFreeBytes = 64 << 20
)

// Settings are what the app's admins set (stored in the app's _params).
type Settings struct {
	Enabled bool   `json:"enabled"`
	Every   string `json:"every"`
}

func (s Settings) every() (time.Duration, error) {
	d, err := time.ParseDuration(s.Every)
	if err != nil || d < time.Hour {
		return 0, fmt.Errorf("every: %q isn't a duration of at least an hour, like 24h or 168h", s.Every)
	}
	return d, nil
}

const settingsParam = "vacuum"

// LoadSettings returns the app's vacuum settings: what its admins saved, else pocketone.yaml's defaults.
func LoadSettings(app core.App) (Settings, error) {
	var raw string
	err := app.DB().NewQuery("SELECT value FROM {{_params}} WHERE id = {:id}").Bind(dbx.Params{"id": settingsParam}).Row(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		c, _ := Section.Get()
		return Settings{Enabled: c.Enabled, Every: c.Every}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Settings{}, fmt.Errorf("vacuum: settings: %w", err)
	}
	return s, nil
}

// SaveSettings stores the app's vacuum settings.
func SaveSettings(app core.App, s Settings) error {
	if _, err := s.every(); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	now := types.NowDateTime().String()
	_, err = app.DB().NewQuery(`INSERT INTO {{_params}} (id, value, created, updated) VALUES ({:id}, {:value}, {:now}, {:now})
		ON CONFLICT(id) DO UPDATE SET value = excluded.value, updated = excluded.updated`).
		Bind(dbx.Params{"id": settingsParam, "value": string(b), "now": now}).Execute()
	return err
}

// Extension compacts the app's databases on its admins' schedule.
type Extension struct{}

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements extkit.Extension.
func (*Extension) Name() string { return Name }

// Contracts implements extkit.Declarer.
func (*Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("vacuum/v1"), WorksWith: extkit.IDs("settings/v1")}
}

// Register implements extkit.Extension: vacuum/v1, a settings/v1 section and the schedule, unless the app's host
// provides vacuum/v1 for its instances (then the host does it, and nothing here shows).
func (*Extension) Register(app core.App, _ extkit.Config) error {
	if extkit.HostProvides(app, vacuumv1.Contract.ID()) {
		return nil
	}
	v := &appVacuum{app: app}
	vacuumv1.Contract.Provide(app, v)
	settingsv1.Add(app, settingsSection{v})
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		v.reschedule()
		return e.Next()
	})
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		v.stop()
		return e.Next()
	})
	return nil
}

// appVacuum is one app's vacuum: its schedule and last run.
type appVacuum struct {
	app core.App

	mu     sync.Mutex
	cancel context.CancelFunc
	last   *vacuumv1.Run
	next   *time.Time
}

var _ vacuumv1.Vacuum = (*appVacuum)(nil)

// Status implements vacuumv1.Vacuum.
func (v *appVacuum) Status(context.Context, extkit.Auth) (vacuumv1.Status, error) {
	s, err := LoadSettings(v.app)
	if err != nil {
		return vacuumv1.Status{}, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return vacuumv1.Status{Provider: Name, Enabled: s.Enabled, Every: s.Every, Last: v.last, Next: v.next}, nil
}

func (v *appVacuum) stop() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cancel != nil {
		v.cancel()
		v.cancel, v.next = nil, nil
	}
}

// reschedule (re)starts the schedule from the current settings.
func (v *appVacuum) reschedule() {
	v.stop()
	s, err := LoadSettings(v.app)
	if err != nil {
		v.app.Logger().Warn("vacuum: settings", "error", err)
		return
	}
	every, err := s.every()
	if !s.Enabled || err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.mu.Lock()
	v.cancel = cancel
	next := time.Now().Add(every)
	v.next = &next
	v.mu.Unlock()
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				v.runOnce()
				v.mu.Lock()
				next := time.Now().Add(every)
				v.next = &next
				v.mu.Unlock()
			}
		}
	}()
}

func (v *appVacuum) runOnce() {
	run := App(v.app)
	v.mu.Lock()
	v.last = &run
	v.mu.Unlock()
	if run.Error != "" {
		v.app.Logger().Warn("vacuum failed", "error", run.Error)
	} else {
		v.app.Logger().Info("vacuum: done", "saved_bytes", run.Saved, "skipped", run.Skipped)
	}
}

// App compacts a running app's databases now (online: writes wait while it runs), each only when enough of it is free
// space ([MinFreeRatio], [MinFreeBytes]).
func App(app core.App) vacuumv1.Run {
	run := vacuumv1.Run{At: time.Now().UTC()}
	dir := app.DataDir()
	for _, d := range []struct {
		file   string
		db     dbx.Builder
		vacuum func() error
	}{
		{"data.db", app.DB(), app.Vacuum},
		{"auxiliary.db", app.AuxDB(), app.AuxVacuum},
	} {
		worth, err := worthIt(d.db)
		if err != nil {
			run.Error = fmt.Sprintf("%s: %v", d.file, err)
			return run
		}
		if !worth {
			run.Skipped = append(run.Skipped, d.file)
			continue
		}
		before, _ := size(filepath.Join(dir, d.file))
		if err := d.vacuum(); err != nil {
			run.Error = fmt.Sprintf("%s: %v", d.file, err)
			return run
		}
		// in WAL mode VACUUM writes the compacted pages to the WAL first: fold them into the file before measuring
		_, _ = d.db.NewQuery("PRAGMA wal_checkpoint(TRUNCATE)").Execute()
		after, _ := size(filepath.Join(dir, d.file))
		run.Saved += before - after
	}
	return run
}

// worthIt reports whether a database has enough free pages to be worth compacting.
func worthIt(db dbx.Builder) (bool, error) {
	var pages, free, pageSize int64
	if err := db.NewQuery("PRAGMA page_count").Row(&pages); err != nil {
		return false, err
	}
	if err := db.NewQuery("PRAGMA freelist_count").Row(&free); err != nil {
		return false, err
	}
	if err := db.NewQuery("PRAGMA page_size").Row(&pageSize); err != nil {
		return false, err
	}
	if pages == 0 {
		return false, nil
	}
	return float64(free)/float64(pages) >= MinFreeRatio || free*pageSize >= MinFreeBytes, nil
}

// size is a database's size with its WAL.
func size(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	n := st.Size()
	if w, err := os.Stat(path + "-wal"); err == nil {
		n += w.Size()
	}
	return n, nil
}

// settingsSection is vacuum's settings/v1 section: on or off, and how often.
type settingsSection struct{ v *appVacuum }

func (s settingsSection) Info() settingsv1.Info {
	return settingsv1.Info{Extension: Name, Title: "Vacuum",
		Description: "Compact the databases on a schedule, so space freed by deleted records goes back to the disk."}
}

func (s settingsSection) Get(context.Context) (json.RawMessage, error) {
	cur, err := LoadSettings(s.v.app)
	if err != nil {
		return nil, err
	}
	return json.Marshal(cur)
}

func (s settingsSection) Save(_ context.Context, values json.RawMessage) (json.RawMessage, error) {
	var next Settings
	if err := json.Unmarshal(values, &next); err != nil {
		return nil, &extkit.Error{Code: "bad_request", Message: `expected {"enabled": true, "every": "168h"}`, Status: 400}
	}
	if err := SaveSettings(s.v.app, next); err != nil {
		return nil, &extkit.Error{Code: "bad_request", Message: err.Error(), Status: 400}
	}
	s.v.reschedule()
	return json.Marshal(next)
}
