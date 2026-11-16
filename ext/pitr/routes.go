package pitr

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	settingsv1 "github.com/pockethost/pocketone/ext/contracts/settings/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

//go:embed ui
var uiFiles embed.FS

// UI returns the admin extension (main.js), mounted on Settings → Backups.
func UI() fs.FS {
	sub, _ := fs.Sub(uiFiles, "ui")
	return sub
}

// PITR has no routes of its own: restores go through restore/v1 and its S3 destination through settings/v1.
func (a *appPITR) onServe(se *core.ServeEvent) error {
	if err := a.attach(); err != nil {
		return err
	}
	se.UIExtensions = append(se.UIExtensions, core.UIExtension{Name: Name, FS: UI()})
	return se.Next()
}

// settingsSection is pitr's settings/v1 section: where its history goes (the app's own S3 bucket).
type settingsSection struct{ a *appPITR }

func (s settingsSection) Info() settingsv1.Info {
	return settingsv1.Info{Extension: Name, Title: "Point-in-time restore",
		Description: "Keep the history in your own S3 bucket instead of the server's storage or disk."}
}

func (s settingsSection) Get(context.Context) (json.RawMessage, error) {
	cur, err := LoadSettings(s.a.app)
	if err != nil {
		return nil, err
	}
	return json.Marshal(toPublic(cur))
}

func (s settingsSection) parse(values json.RawMessage) (Settings, error) {
	prev, err := LoadSettings(s.a.app)
	if err != nil {
		return Settings{}, err
	}
	var next Settings
	if err := json.Unmarshal(values, &next); err != nil {
		return Settings{}, &extkit.Error{Code: "bad_request", Message: `expected {"s3": {...}}`, Status: 400}
	}
	return mergeSecret(next, prev), nil
}

func (s settingsSection) Save(_ context.Context, values json.RawMessage) (json.RawMessage, error) {
	next, err := s.parse(values)
	if err != nil {
		return nil, err
	}
	if err := SaveSettings(s.a.app, next); err != nil {
		return nil, &extkit.Error{Code: "bad_request", Message: err.Error(), Status: 400}
	}
	// the history keeps going, now to the new destination (a fresh snapshot there)
	if err := s.a.reattach(); err != nil {
		return nil, &extkit.Error{Code: "reattach_failed", Message: "saved, but replication didn't restart: " + err.Error(), Status: 500}
	}
	return json.Marshal(toPublic(next))
}

func (s settingsSection) Test(_ context.Context, values json.RawMessage) error {
	next, err := s.parse(values)
	if err != nil {
		return err
	}
	return testS3(next.S3)
}
