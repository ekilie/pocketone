// Package settingsv1 is the settings/v1 contract: the settings an app's admins change for its extensions (pitr's S3
// bucket, bulkmail's sending limits), one API for all of them. Each extension registers its section with [Add]; the
// contract's provider (ext/settings) lists them, reads, saves and tests them.
//
// A wire contract at /api/_c/settings/v1. Settings an operator sets for a whole host stay in pocketone.yaml; these are
// the ones the app's own superusers change.
package settingsv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"pocketone.dev/ext/extkit"
)

// Settings reads and changes the app's extension settings.
//
//contract:id settings/v1
//contract:kind wire
//contract:scope settings
//contract:host
//contract:js hooks
type Settings interface {
	// List names the extensions whose settings this app's admins can change.
	//
	//op: GET / scope=settings:read
	List(ctx context.Context, a extkit.Auth) (List, error)

	// Get returns one extension's settings. Secrets come back empty, with a flag saying they're set.
	//
	//op: GET get scope=settings:read
	Get(ctx context.Context, a extkit.Auth, in Ref) (Values, error)

	// Save replaces one extension's settings and applies them. A secret left empty keeps its saved value. Settings a
	// host sets are read-only here.
	//
	//op: POST save scope=settings
	Save(ctx context.Context, a extkit.Auth, in Change) (Values, error)

	// Test checks one extension's settings without saving them (a test write to a bucket, a test message).
	//
	//op: POST test scope=settings
	Test(ctx context.Context, a extkit.Auth, in Change) (TestResult, error)
}

// Info describes one extension's settings.
type Info struct {
	Extension   string `json:"extension"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Testable    bool   `json:"testable"`
	ReadOnly    bool   `json:"readOnly"` // shown, not changed here (a host sets them)
}

// List is the extensions with settings.
type List struct {
	Sections []Info `json:"sections"`
}

// Ref names an extension.
type Ref struct {
	Extension string `json:"extension"`
}

// Values are one extension's settings, in its own shape.
type Values struct {
	Extension string          `json:"extension"`
	Values    json.RawMessage `json:"values"`
}

// Change is settings to save or test.
type Change struct {
	Extension string          `json:"extension"`
	Values    json.RawMessage `json:"values"`
}

// TestResult says how a test went.
type TestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ErrUnknown: no extension with that name has settings on this app.
var ErrUnknown = &extkit.Error{Code: "unknown_extension", Message: "no settings for that extension on this app", Status: 404}

// ErrReadOnly: the extension's settings are shown here but set elsewhere (by the host).
var ErrReadOnly = &extkit.Error{Code: "read_only", Message: "these settings are set by the host", Status: 400}

// ErrNotTestable: the extension's settings have no test.
var ErrNotTestable = &extkit.Error{Code: "not_testable", Message: "these settings have no test", Status: 400}

// Section is one extension's settings, as its extension keeps them. A section the admins can change is also a
// [Saver]; one they can only see (set by the host) isn't.
type Section interface {
	Info() Info
	Get(ctx context.Context) (json.RawMessage, error)
}

// Saver is a Section whose settings the admins change. Return an *extkit.Error for a value they should fix.
type Saver interface {
	Save(ctx context.Context, values json.RawMessage) (json.RawMessage, error)
}

// Tester is a Section that can check settings without saving them.
type Tester interface {
	Test(ctx context.Context, values json.RawMessage) error
}

const storeKey = "settings/v1.sections"

type registry struct {
	mu       sync.Mutex
	sections map[string]Section
}

func registryOf(app core.App) *registry {
	return app.Store().GetOrSet(storeKey, func() any { return &registry{sections: map[string]Section{}} }).(*registry)
}

// Add registers an extension's settings on app (call it in Register). The provider of settings/v1 serves them.
func Add(app core.App, s Section) {
	r := registryOf(app)
	r.mu.Lock()
	r.sections[s.Info().Extension] = s
	r.mu.Unlock()
}

// Sections returns app's registered sections, sorted by extension name (for providers).
func Sections(app core.App) []Section {
	r := registryOf(app)
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Section, 0, len(r.sections))
	for _, s := range r.sections {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Info().Extension < out[j].Info().Extension })
	return out
}

// SectionOf returns one extension's section on app.
func SectionOf(app core.App, extension string) (Section, bool) {
	r := registryOf(app)
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sections[extension]
	return s, ok
}
