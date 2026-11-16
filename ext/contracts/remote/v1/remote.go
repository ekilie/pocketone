// Package remotev1 is the remote/v1 contract: remote control of an app. It's the base every client can rely on (who
// you are, the app's status, restart, logs); contracts that add an area, like deploy/v1, require it.
//
// A wire contract at /api/_c/remote/v1; whoami at its root is the one fixed path a client starts from. A standalone
// app gets it from ext/remotes; a multisite host provides it for its instances (so it works for stock images too).
package remotev1

//go:generate go run github.com/pockethost/pocketone/ext/contracts/cmd/contractgen

import (
	"context"

	"github.com/pockethost/pocketone/ext/extkit"
)

// Remote controls an app from afar.
//
//contract:id remote/v1
//contract:kind wire
//contract:scope remote
//contract:host
//contract:js hooks
type Remote interface {
	// Whoami says who the credential is, the app's name and URL, and the contracts it offers.
	//
	//op: GET / scope=public
	Whoami(ctx context.Context, a extkit.Auth) (Whoami, error)

	// Status reports whether the app is running, its versions, uptime and mode.
	//
	//op: GET status scope=remote:read
	Status(ctx context.Context, a extkit.Auth) (Status, error)

	// Restart restarts the app; requests in flight finish first.
	//
	//op: POST restart scope=remote
	Restart(ctx context.Context, a extkit.Auth) (RestartResult, error)

	// Logs streams the app's log: the last entries, then new ones as they're written.
	//
	//op: GET logs scope=remote:read
	Logs(ctx context.Context, a extkit.Auth, in LogsInput) (extkit.Stream, error)
}

// Whoami is who a credential is and what the app offers.
type Whoami struct {
	Auth *Identity `json:"auth"` // nil for an anonymous caller
	// Via: "token" (API token), "jwt", or "" (anonymous).
	Via     string   `json:"via"`
	Scopes  []string `json:"scopes"` // a token's scopes; empty: the owner's full rights
	TokenID string   `json:"tokenId,omitempty"`
	App     App      `json:"app"`
	// Contracts are the wire contracts the app offers ("deploy/v1"): each at /api/_c/<id>/.
	Contracts []string `json:"contracts"`
	// Extensions are the extensions running on the app; only for a signed-in caller.
	Extensions []string `json:"extensions,omitempty"`
}

// Identity is the record a credential acts as.
type Identity struct {
	Collection string `json:"collection"`
	ID         string `json:"id"`
	Email      string `json:"email,omitempty"`
}

// App names the app.
type App struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Status is how the app is doing.
type Status struct {
	Running    bool   `json:"running"`
	Version    string `json:"version"`    // the PocketOne build ("0.0.2", "dev+87f81fd2")
	PocketBase string `json:"pocketbase"` // the PocketBase version it's based on
	Uptime     string `json:"uptime"`     // since this app last started ("3h2m5s")
	Dev        bool   `json:"dev"`
}

// RestartResult says what a restart did.
type RestartResult struct {
	Restarting bool   `json:"restarting"`
	Message    string `json:"message,omitempty"`
}

// LogsInput picks how much history a log stream starts with.
type LogsInput struct {
	N int `json:"n"` // entries to replay first (default 100, at most 1000)
}

// LogEntry is one line of an app's log.
type LogEntry struct {
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Stream  string         `json:"stream,omitempty"` // stdout or stderr, for process output
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}
