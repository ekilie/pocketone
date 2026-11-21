// Package tokensv1 is the instance.tokens/v1 contract: a multisite host checks an API token one of its instances
// issued, so it can accept the instance's own tokens on the routes it answers for that instance (deploy/v1,
// remote/v1). `pocketone login` and `clone` sign in to the instance (its oauth/v1), so this is how a host lets them
// in the same way on every host.
//
// A service contract a runtime provides on the host scope (ProvideIn), for the instances it runs: Pocker checks the
// token with the instance's own api-tokens/v1. A runtime whose instances it can't reach inside (Docker's stock images)
// doesn't provide it, and the host accepts only its own credentials there.
package tokensv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"errors"

	verifyv1 "pocketone.dev/ext/contracts/api-tokens/verify/v1"
)

// Tokens checks the API tokens a host's instances issued.
//
//contract:id instance.tokens/v1
//contract:kind service
type Tokens interface {
	// Verify checks a token with the instance's own API tokens. A token the instance didn't issue (or revoked, or that
	// expired) is an answer (Valid false), not an error. ErrNoInstance: the provider doesn't run that instance, or the
	// instance has no API tokens. It may start an instance that isn't running.
	Verify(ctx context.Context, in Input) (verifyv1.Result, error)
}

// Input names the instance and the token.
type Input struct {
	Instance string `json:"instance"` // the instance id
	Token    string `json:"token"`    // the secret the client sent ("pbt_...")
}

// ErrNoInstance: the provider doesn't run that instance, or it has no API tokens.
var ErrNoInstance = errors.New("instance.tokens/v1: no such instance, or it has no API tokens")
