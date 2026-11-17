// Package clearv1 is the cache.clear/v1 contract: empty an app's cache buckets from outside the app's code, so its
// admins (the dashboard, `pocketone remote cache clear`, an agent) can drop what cache/v1 keeps for it after a change
// that left stale entries behind.
//
// A wire contract at /api/_c/cache.clear/v1, beside cache/v1: whoever provides cache/v1 provides this too.
package clearv1

//go:generate go run github.com/pockethost/pocketone/ext/contracts/cmd/contractgen

import (
	"context"

	"github.com/pockethost/pocketone/ext/extkit"
)

// Clearer empties an app's cache buckets.
//
//contract:id cache.clear/v1
//contract:kind wire
//contract:scope cache
type Clearer interface {
	// Clear empties one of the app's cache buckets, or every bucket it has.
	//
	//op: POST / scope=cache
	Clear(ctx context.Context, a extkit.Auth, in ClearInput) (ClearResult, error)
}

// ClearInput says what to empty.
type ClearInput struct {
	Bucket string `json:"bucket,omitempty"` // one bucket by name; "" empties every bucket of the app
}

// ClearResult says what went.
type ClearResult struct {
	Keys int `json:"keys"` // keys removed
}
