// Package vacuum compacts an app's SQLite databases on a schedule (VACUUM), so space freed by deleted records goes back
// to the disk.
package vacuum

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's key.
const Name = "vacuum"

// Config is the `vacuum` section of pocketone.yaml. The section turns vacuum on.
//
//	vacuum:
//	  every: 168h   # how often (default weekly)
type Config struct {
	Every string `yaml:"every" doc:"How often to compact the databases, e.g. 24h or 168h (weekly). Default 168h."`
}

// Section is the `vacuum` section.
var Section = extkit.NewSection(Name, Config{Every: "168h"}, "Compact the app's SQLite databases on a schedule (VACUUM). Turns vacuum on.")

func (c Config) every() (time.Duration, error) {
	d, err := time.ParseDuration(c.Every)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("vacuum: every: %q is not a positive duration like 168h", c.Every)
	}
	return d, nil
}

// Databases are the files compacted in an app's pb_data.
var Databases = []string{"data.db", "auxiliary.db"}

// Extension compacts a single app's databases on the `vacuum` section's schedule, online.
type Extension struct{}

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements extkit.Extension.
func (*Extension) Name() string { return Name }

// Register implements extkit.Extension: with a `vacuum` section, a background loop runs [App] every `every` while the
// app serves.
func (*Extension) Register(app core.App, _ extkit.Config) error {
	c, ok := Section.Get()
	if !ok {
		return nil
	}
	every, err := c.every()
	if err != nil {
		return err
	}
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		ctx, stop := context.WithCancel(context.Background())
		app.OnTerminate().BindFunc(func(t *core.TerminateEvent) error { stop(); return t.Next() })
		go loop(ctx, every, func(context.Context) {
			if err := App(app); err != nil {
				app.Logger().Warn("vacuum failed", "error", err)
			}
		})
		return e.Next()
	})
	return nil
}

// App compacts a running app's databases online (PocketBase's VACUUM on its main and auxiliary database).
func App(app core.App) error {
	start := time.Now()
	if err := app.Vacuum(); err != nil {
		return fmt.Errorf("data.db: %w", err)
	}
	if err := app.AuxVacuum(); err != nil {
		return fmt.Errorf("auxiliary.db: %w", err)
	}
	app.Logger().Info("vacuum: done", "ms", time.Since(start).Milliseconds())
	return nil
}

// Dir compacts the databases in an app folder's pb_data while nothing has them open (the app isn't running), and
// returns the bytes saved. Missing databases are skipped.
func Dir(appDir string) (saved int64, err error) {
	for _, name := range Databases {
		path := filepath.Join(appDir, "pb_data", name)
		before, err := size(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return saved, err
		}
		db, err := core.DefaultDBConnect(path)
		if err != nil {
			return saved, fmt.Errorf("%s: %w", name, err)
		}
		_, err = db.NewQuery("VACUUM").Execute()
		_ = db.Close()
		if err != nil {
			return saved, fmt.Errorf("%s: %w", name, err)
		}
		after, _ := size(path)
		saved += before - after
	}
	return saved, nil
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

// loop runs fn every interval until ctx is done (the first run after one interval).
func loop(ctx context.Context, every time.Duration, fn func(context.Context)) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn(ctx)
		}
	}
}
