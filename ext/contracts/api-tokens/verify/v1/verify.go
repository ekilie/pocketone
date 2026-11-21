// Package verifyv1 is the api-tokens.verify/v1 contract: another server checks an app's API token. A host in front
// of its apps (a PocketHost edge in front of the mothership's accounts) is handed a token by a client and asks the app
// that issued it who the token acts as and what it may do. Whatever provides api-tokens/v1 provides it too.
//
// A wire contract at /api/_c/api-tokens.verify/v1, for superusers only: it answers for any token.
package verifyv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	apitokensv1 "pocketone.dev/ext/contracts/api-tokens/v1"
	"pocketone.dev/ext/extkit"
)

// Verifier checks an app's API tokens for another server.
//
//contract:id api-tokens.verify/v1
//contract:kind wire
//contract:scope api_tokens_verify
type Verifier interface {
	// Verify checks an API token and says who it acts as and what it may do (its scopes, with their values). A token
	// that isn't valid is an answer (Valid false), not an error, so callers can tell a refused token from an outage.
	//
	//op: POST /
	Verify(ctx context.Context, a extkit.Auth, in Input) (Result, error)
}

// Input is the token to check.
type Input struct {
	Token string `json:"token"` // the secret the client sent ("pbt_...")
	// IP is the client's address, recorded as the token's last use; "" leaves the last use alone.
	IP string `json:"ip,omitempty"`
}

// Result is whether the token is valid and, when it is, what it grants.
type Result struct {
	Valid bool `json:"valid"`
	// Reason says why a token isn't valid: "invalid" (unknown, malformed or revoked) or "expired".
	Reason string             `json:"reason,omitempty"`
	Grant  *apitokensv1.Grant `json:"grant,omitempty"`
}

// Reasons a token isn't valid.
const (
	Invalid = "invalid"
	Expired = "expired"
)
