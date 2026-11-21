package pocker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	apitokensv1 "pocketone.dev/ext/contracts/api-tokens/v1"
	"pocketone.dev/ext/contracts/instance/source/v1/sourcetest"
	tokensv1 "pocketone.dev/ext/contracts/instance/tokens/v1"
)

// fakeIssuer is an instance's api-tokens/v1 (pocker can't import ext/api-tokens): it knows one token per instance.
type fakeIssuer struct{ id string }

func (f fakeIssuer) Verify(_ context.Context, secret string) (apitokensv1.Grant, error) {
	switch secret {
	case "pbt_" + f.id:
		return apitokensv1.Grant{TokenID: "t1", OwnerCollection: core.CollectionNameSuperusers, OwnerID: "su1",
			Scopes: []string{"deploy"}}, nil
	case "pbt_old":
		return apitokensv1.Grant{}, apitokensv1.ErrExpired
	}
	return apitokensv1.Grant{}, apitokensv1.ErrInvalid
}
func (fakeIssuer) RegisterScope(apitokensv1.Scope) error             { return nil }
func (fakeIssuer) Scopes() []apitokensv1.Scope                       { return nil }
func (fakeIssuer) Reaches(scopes []string, method, path string) bool { return false }

func TestTokensVerifyWithTheInstance(t *testing.T) {
	_, scope := sourcetest.Scope(Instance{ID: "a"}, Instance{ID: "b"}, Instance{ID: "bare"})
	m := NewManager(scope, ManagerOptions{Standard: testStandard, PBRoot: t.TempDir(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Init: func(inst Instance, app core.App) error {
			if inst.ID != "bare" { // "bare" has no API tokens
				apitokensv1.Contract.Provide(app, fakeIssuer{inst.ID})
			}
			return nil
		}})
	runManager(t, m)
	tk := m.Tokens()
	ctx := context.Background()

	res, err := tk.Verify(ctx, tokensv1.Input{Instance: "a", Token: "pbt_a"})
	if err != nil || !res.Valid || res.Grant == nil || res.Grant.OwnerID != "su1" || res.Grant.Scopes[0] != "deploy" {
		t.Fatalf("a's own token: %+v %v", res, err)
	}
	if !m.Running("a") {
		t.Fatal("verifying should have started the instance")
	}
	if res, err := tk.Verify(ctx, tokensv1.Input{Instance: "b", Token: "pbt_a"}); err != nil || res.Valid || res.Reason != "invalid" {
		t.Fatalf("a's token on b: %+v %v", res, err)
	}
	if res, err := tk.Verify(ctx, tokensv1.Input{Instance: "a", Token: "pbt_old"}); err != nil || res.Valid || res.Reason != "expired" {
		t.Fatalf("expired: %+v %v", res, err)
	}
	if _, err := tk.Verify(ctx, tokensv1.Input{Instance: "nope", Token: "pbt_a"}); !errors.Is(err, tokensv1.ErrNoInstance) {
		t.Fatalf("unknown instance: %v", err)
	}
	if _, err := tk.Verify(ctx, tokensv1.Input{Instance: "bare", Token: "pbt_bare"}); !errors.Is(err, tokensv1.ErrNoInstance) {
		t.Fatalf("instance without tokens: %v", err)
	}
}
