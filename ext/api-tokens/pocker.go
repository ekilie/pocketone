package apitokens

import (
	"context"
	_ "embed"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	apitokensv1 "pocketone.dev/ext/contracts/api-tokens/v1"
	"pocketone.dev/ext/extkit"
)

// Name is the extension's key under an instance's `extensions:`.
const Name = "api-tokens"

// Extension is API tokens as a PocketOne extension, a member of ext/standard's set: every app has it, and on a
// multisite host every instance that runs PocketOne gets its own _api_tokens_tokens, routes and screen. Reconfigure it on the set:
//
//	set.APITokens = apitokens.New(apitokens.Config{})
//
// With Pocker, per instance, `extensions: {api-tokens: {enabled: false}}` turns it off and `collections: [...]` picks who may own
// tokens. Scopes are registered per app by the extensions that own them (they call apitokens.For(app).RegisterScope).
type Extension struct{ cfg Config }

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension. cfg holds the defaults for every app.
func New(cfg Config) *Extension { return &Extension{cfg: cfg} }

// Name implements [extkit.Extension].
func (p *Extension) Name() string { return Name }

type settings struct {
	Collections []string `json:"collections"`
}

// Register implements [extkit.Extension].
func (p *Extension) Register(app core.App, layers extkit.Config) error {
	cfg := p.cfg
	var s settings
	if err := layers.Decode(&s); err != nil {
		return err
	}
	if len(s.Collections) > 0 {
		cfg.Collections = s.Collections
	}
	_, err := Register(app, cfg)
	return err
}

// Contracts implements [extkit.Declarer].
func (p *Extension) Contracts() extkit.Contracts {
	c := extkit.Contracts{Implements: extkit.IDs("api-tokens/v1", "api-tokens.verify/v1")}
	if !p.cfg.OAuth.Disabled {
		c.Implements = append(c.Implements, "oauth/v1") // the OAuth server comes with tokens
		c.WorksWith = extkit.IDs("templates/v1")        // its pages are a template set a project can eject
	}
	return c
}

// issuer is api-tokens/v1 over this app's tokens.
type issuer struct{ t *Tokens }

func (i issuer) Verify(_ context.Context, secret string) (apitokensv1.Grant, error) {
	info, err := i.t.Verify(secret)
	if err != nil {
		return apitokensv1.Grant{}, err
	}
	return info.grant(), nil
}

func (i issuer) RegisterScope(s apitokensv1.Scope) error { return i.t.RegisterScope(Scope(s)) }

func (i issuer) Scopes() []apitokensv1.Scope {
	list := i.t.Scopes()
	out := make([]apitokensv1.Scope, len(list))
	for n, s := range list {
		out[n] = apitokensv1.Scope(s)
	}
	return out
}

func (i issuer) Reaches(scopes []string, method, path string) bool {
	return i.t.Reaches(scopes, method, path)
}

// grant is the token as api-tokens/v1 describes it.
func (info *Info) grant() apitokensv1.Grant {
	return apitokensv1.Grant{
		TokenID:         info.Token.Id,
		Name:            info.Token.GetString("name"),
		OwnerCollection: info.Owner.Collection().Name,
		OwnerID:         info.Owner.Id,
		Scopes:          info.Scopes,
		Audience:        info.Audience,
		ClientID:        info.Token.GetString("client_id"),
		Owner:           info.Owner,
	}
}

//go:embed jsbind.d.ts
var jsTypes string

// TypeScript declares the extension's JS API for types.d.ts (extkit/js Typer).
func (p *Extension) TypeScript() string { return jsTypes }
