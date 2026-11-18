// Package runtime runs a PocketOne app as a desktop app: the app's server on a private 127.0.0.1 port, and a native
// window (Wails, the system webview) showing it. `pocketone desktop build` compiles a main package that embeds the
// app's pb_hooks, pb_migrations and pb_public and calls [Run]; it is its own module so the `pocketone` binary never
// links Wails.
package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/bundle"
	"github.com/pockethost/pocketone/ext/jsvm"
	"github.com/pockethost/pocketone/ext/standard"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// App is a desktop app.
type App struct {
	Name string // the window title and the app's name in the OS
	ID   string // reverse-DNS id ("dev.pocketone.hello"): names the data folder and the single-instance lock

	Width, Height int // the window's starting size; 0 for 1024 × 768

	// the app's files, nil for none: pb_hooks, pb_migrations and pb_public, as `pocketone serve` reads them from disk
	Hooks, Migrations, Public fs.FS

	// DataDir overrides where pb_data lives; default <the OS's per-user config folder>/<ID>/pb_data
	// (~/Library/Application Support on macOS, %AppData% on Windows, ~/.config on Linux)
	DataDir string
}

// Run starts the app's server and its window, and returns when the window closes. It exits the process on an error.
func Run(a App) {
	if err := run(a); err != nil {
		log.Fatal(err)
	}
}

func run(a App) error {
	if a.ID == "" || a.Name == "" {
		return errors.New("desktop: App needs a Name and an ID")
	}
	dataDir := a.DataDir
	if dataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("desktop: no per-user config folder: %w", err)
		}
		dataDir = filepath.Join(base, a.ID, "pb_data")
	}

	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: dataDir, HideStartBanner: true})

	// the standard set, with the app's hooks and migrations read from the binary, never from disk
	var exts []any
	if a.Hooks != nil {
		exts = append(exts, bundle.Hooks(a.Hooks))
	}
	if a.Migrations != nil {
		exts = append(exts, bundle.Migrations(a.Migrations))
	}
	if err := standard.New().Register(app, jsvm.Config{Extensions: exts}); err != nil {
		return err
	}
	if a.Public != nil {
		bundle.Public(app, a.Public, true)
	}
	// a desktop app's users aren't its superusers: never open the browser on the superuser installer
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.InstallerFunc = nil
		return e.Next()
	})

	if err := app.Bootstrap(); err != nil {
		return err
	}
	addr, err := freeAddr()
	if err != nil {
		return err
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- apis.Serve(app, apis.ServeConfig{HttpAddr: addr})
	}()
	base := "http://" + addr
	if err := waitHealthy(base, serveErr); err != nil {
		return err
	}
	slog.Info("desktop: serving", "url", base, "data", dataDir)

	width, height := a.Width, a.Height
	if width == 0 || height == 0 {
		width, height = 1024, 768
	}
	win := application.New(application.Options{
		Name:           a.Name,
		SingleInstance: &application.SingleInstanceOptions{UniqueID: a.ID},
		Mac:            application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	win.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  a.Name,
		Width:  width,
		Height: height,
		URL:    base,
	})
	err = win.Run()

	// the window is gone: let the app's terminate hooks (pitr, open stores) run
	_ = app.OnTerminate().Trigger(&core.TerminateEvent{App: app}, func(e *core.TerminateEvent) error {
		return e.App.ResetBootstrapState()
	})
	return err
}

// freeAddr is a free 127.0.0.1 port, so the app never collides with another server (or another copy of itself).
func freeAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return l.Addr().String(), nil
}

// waitHealthy waits up to 30 s for the server's /api/health, failing at once if the server stops.
func waitHealthy(base string, serveErr <-chan error) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-serveErr:
			return fmt.Errorf("desktop: the server stopped: %w", err)
		default:
		}
		if res, err := client.Get(base + "/api/health"); err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("desktop: the server didn't answer /api/health within 30 s")
}
