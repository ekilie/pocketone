package apitokens

import (
	"context"
	"errors"

	verifyv1 "pocketone.dev/ext/contracts/api-tokens/verify/v1"
	"pocketone.dev/ext/extkit"
)

// verifier is api-tokens.verify/v1 over this app's tokens: another server (a host in front of this app's accounts)
// checks a token a client handed it.
type verifier struct{ t *Tokens }

func (v verifier) Verify(_ context.Context, _ extkit.Auth, in verifyv1.Input) (verifyv1.Result, error) {
	info, err := v.t.Verify(in.Token)
	switch {
	case errors.Is(err, ErrInvalid):
		return verifyv1.Result{Reason: verifyv1.Invalid}, nil
	case errors.Is(err, ErrExpired):
		return verifyv1.Result{Reason: verifyv1.Expired}, nil
	case err != nil:
		return verifyv1.Result{}, err
	}
	if in.IP != "" {
		v.t.touch(info.Token, in.IP)
	}
	g := info.grant()
	if g.Scopes == nil {
		g.Scopes = []string{}
	}
	return verifyv1.Result{Valid: true, Grant: &g}, nil
}
