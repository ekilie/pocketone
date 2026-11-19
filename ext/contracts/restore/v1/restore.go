// Package restorev1 is the restore/v1 contract: put an app's data back as it was at a moment. Its providers differ in
// how close they get: pitr restores any second in its window (continuous history); the default restores the latest
// backup at or before the moment. Both answer the same calls, so a client (the dashboard, the CLI, an agent) asks for
// "Tuesday 15:00" and gets the closest the app can do, and is told which point it got.
//
// A wire contract at /api/_c/restore/v1; a multisite host may provide it for its instances (from host-level
// snapshots).
package restorev1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"time"

	"pocketone.dev/ext/extkit"
)

// Restorer puts an app's data back as it was at a moment.
//
//contract:id restore/v1
//contract:kind wire
//contract:scope restore
//contract:host
//contract:js hooks
type Restorer interface {
	// Points says what the app's data can be restored to: a window of time (continuous history) or a list of
	// moments (backups), and whether a restore is running.
	//
	//op: GET / scope=restore:read
	Points(ctx context.Context, a extkit.Auth) (Points, error)

	// RestoreTo puts the app's data back as it was at the given moment, from the latest point at or before it. The
	// app restarts to apply it; its current data is kept aside until the next restore.
	//
	//op: POST restore scope=restore
	RestoreTo(ctx context.Context, a extkit.Auth, in RestoreInput) (RestoreResult, error)
}

// Kinds of points.
const (
	Continuous = "continuous" // any moment inside Window
	Snapshots  = "snapshots"  // only the moments in Snapshots
)

// Points is what the app can be restored to.
type Points struct {
	Provider  string     `json:"provider"` // who answers: "pitr", "backups", a host
	Kind      string     `json:"kind"`     // Continuous or Snapshots
	Window    *Window    `json:"window,omitempty"`
	Snapshots []Snapshot `json:"snapshots,omitempty"` // newest first
	Restoring bool       `json:"restoring"`
	Last      *Restored  `json:"last,omitempty"` // the last restore, if any
	// Details are the provider's own facts (pitr: where the history is kept and how big it is), for display only.
	Details map[string]any `json:"details,omitempty"`
}

// Window is a span of time any moment of which can be restored.
type Window struct {
	Earliest time.Time `json:"earliest"`
	Latest   time.Time `json:"latest"`
}

// Snapshot is one moment that can be restored.
type Snapshot struct {
	At   time.Time `json:"at"`
	Name string    `json:"name"` // the provider's name for it (a backup file)
	Size int64     `json:"size,omitempty"`
}

// Restored is a finished restore.
type Restored struct {
	Asked time.Time `json:"asked"` // the moment asked for
	Point time.Time `json:"point"` // the point the data came from
	Done  time.Time `json:"done"`
}

// RestoreInput is the moment to restore to.
type RestoreInput struct {
	At time.Time `json:"at"`
}

// RestoreResult says what a restore will use.
type RestoreResult struct {
	Restoring bool      `json:"restoring"`
	Point     time.Time `json:"point"` // the point it restores from: At itself (continuous) or the latest snapshot before it
	Name      string    `json:"name,omitempty"`
	Message   string    `json:"message"`
}

// ErrNoPoint: nothing to restore from at or before the moment asked for.
var ErrNoPoint = &extkit.Error{Code: "no_point", Message: "nothing to restore from at or before that moment", Status: 422}

// ErrBusy: a restore is already running.
var ErrBusy = &extkit.Error{Code: "busy", Message: "a restore is already running", Status: 409}
