// Package kvv1 is the kv/v1 contract: an app's key-value store. Named JSON values (an app's settings, an extension's
// state) kept without a collection or a migration, in PocketBase's own _params table, so they ride along in backups.
//
// Two halves:
//
//   - The helpers in this package ([Get], [Set], [Delete], [List]) are how Go code (an extension, a hook) reads and
//     writes the store. They work on the app's database directly, inside a transaction when given one, and need no
//     provider.
//   - The wire API at /api/_c/kv/v1 (superusers only) is how the CLI, MCP and a dashboard read and change values. Its
//     provider (ext/kv) serves it over the same helpers, and gives page code $kv.
//
// Each value is one _params row whose id is [Prefix] + its name ("kv:storytime/settings"), so a name can never meet
// PocketBase's own params ("settings") or another extension's rows. Names are 1 to [MaxNameLength] characters: letters,
// digits and _ - . : / , starting with a letter or digit. Use a prefix of your own ("myext/...") to keep your names
// apart from others'.
package kvv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"encoding/json"
	"time"

	"pocketone.dev/ext/extkit"
)

// Store reads and changes the app's key-value store.
//
//contract:id kv/v1
//contract:kind wire
//contract:scope kv
type Store interface {
	// List returns the values whose names start with prefix (all of them when it's empty), sorted by name.
	//
	//op: GET / scope=kv:read
	List(ctx context.Context, a extkit.Auth, in ListInput) (Listing, error)

	// Get returns one value. A name with no value is a not_found error.
	//
	//op: GET get scope=kv:read
	Get(ctx context.Context, a extkit.Auth, in Key) (Entry, error)

	// Set stores a JSON value under a name, replacing any value it had.
	//
	//op: POST set scope=kv
	Set(ctx context.Context, a extkit.Auth, in SetInput) (Entry, error)

	// Delete removes a value. Deleting a name with no value is not an error.
	//
	//op: POST delete scope=kv
	Delete(ctx context.Context, a extkit.Auth, in Key) error
}

// Key names a value.
type Key struct {
	Name string `json:"name"`
}

// ListInput picks the values to list.
type ListInput struct {
	Prefix string `json:"prefix,omitempty"`
}

// SetInput is a value to store.
type SetInput struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

// Entry is one stored value.
type Entry struct {
	Name    string          `json:"name"`
	Value   json.RawMessage `json:"value"`
	Created time.Time       `json:"created"`
	Updated time.Time       `json:"updated"`
}

// Listing is the values [Store.List] found.
type Listing struct {
	Entries []Entry `json:"entries"`
}

// ErrNotFound: no value under that name.
var ErrNotFound = &extkit.Error{Code: "not_found", Message: "no value under that name", Status: 404}

// ErrInvalidName: the name is empty, too long or has a character names can't have.
var ErrInvalidName = &extkit.Error{Code: "invalid_name", Message: "a name is 1 to 128 characters: letters, digits and _ - . : /, starting with a letter or digit", Status: 400}

// ErrInvalidValue: the value isn't JSON, or is bigger than [MaxValueBytes].
var ErrInvalidValue = &extkit.Error{Code: "invalid_value", Message: "the value must be JSON of at most 1 MiB", Status: 400}
