// Package vacuumv1 is the vacuum/v1 contract: an app's databases are compacted (SQLite VACUUM) so space freed by deleted
// records goes back to the disk, and its admins can see when that last happened. On a single app the vacuum extension
// does it, on a schedule its admins set; a multisite host may provide it for its instances instead, compacting them
// on its own schedule, and then the app's admins see its status but have no setting.
//
// A wire contract at /api/_c/vacuum/v1.
package vacuumv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"time"

	"pocketone.dev/ext/extkit"
)

// Vacuum reports on compacting the app's databases.
//
//contract:id vacuum/v1
//contract:kind wire
//contract:scope vacuum
//contract:host
type Vacuum interface {
	// Status says who compacts the app's databases, whether and how often, and how the last run went.
	//
	//op: GET / scope=vacuum:read
	Status(ctx context.Context, a extkit.Auth) (Status, error)
}

// Status is the app's vacuum status.
type Status struct {
	Provider string     `json:"provider"` // who compacts: "vacuum" (the app itself) or a host
	Enabled  bool       `json:"enabled"`
	Every    string     `json:"every,omitempty"` // how often, e.g. "168h"
	Last     *Run       `json:"last,omitempty"`
	Next     *time.Time `json:"next,omitempty"`
}

// Run is one pass over the app's databases.
type Run struct {
	At      time.Time `json:"at"`
	Saved   int64     `json:"saved"`             // bytes given back to the disk
	Skipped []string  `json:"skipped,omitempty"` // databases with too little free space to be worth it
	Error   string    `json:"error,omitempty"`
}
