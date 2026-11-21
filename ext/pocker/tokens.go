package pocker

import (
	"context"
	"errors"

	apitokensv1 "pocketone.dev/ext/contracts/api-tokens/v1"
	verifyv1 "pocketone.dev/ext/contracts/api-tokens/verify/v1"
	tokensv1 "pocketone.dev/ext/contracts/instance/tokens/v1"
)

// Tokens is instance.tokens/v1 for the Manager's instances: a token is checked with the instance's own api-tokens/v1,
// starting the instance if it isn't running.
func (m *Manager) Tokens() tokensv1.Tokens { return managerTokens{m} }

type managerTokens struct{ m *Manager }

func (t managerTokens) Verify(ctx context.Context, in tokensv1.Input) (verifyv1.Result, error) {
	m := t.m
	m.mu.RLock()
	inst, ok := m.dir.Get(in.Instance)
	m.mu.RUnlock()
	if !ok {
		return verifyv1.Result{}, tokensv1.ErrNoInstance
	}
	if err := m.StartInstance(ctx, in.Instance); err != nil { // waits out maintenance; refuses a blocked instance
		return verifyv1.Result{}, err
	}
	ra, err := m.app(inst)
	if err != nil {
		return verifyv1.Result{}, err
	}
	iss, ok := apitokensv1.Contract.Of(ra.app)
	if !ok {
		return verifyv1.Result{}, tokensv1.ErrNoInstance
	}
	g, err := iss.Verify(ctx, in.Token)
	switch {
	case errors.Is(err, apitokensv1.ErrInvalid):
		return verifyv1.Result{Reason: verifyv1.Invalid}, nil
	case errors.Is(err, apitokensv1.ErrExpired):
		return verifyv1.Result{Reason: verifyv1.Expired}, nil
	case err != nil:
		return verifyv1.Result{}, err
	}
	if g.Scopes == nil {
		g.Scopes = []string{}
	}
	return verifyv1.Result{Valid: true, Grant: &g}, nil
}
