// Package apitokensv1 is the api-tokens/v1 contract: an app's API tokens. ext/api-tokens provides it. In Go, [Issuer] checks a
// token; over HTTP, its API at /api/_c/api-tokens/v1 lists, creates and revokes them (a routed contract):
//
//	GET    /api/_c/api-tokens/v1/scopes      the scopes a token may hold, and who may own tokens
//	GET    /api/_c/api-tokens/v1             your tokens (superusers: ?owner=, ?ownerCollection=)
//	POST   /api/_c/api-tokens/v1             create one ({name, scopes, expires|ttl}); the secret is shown once
//	DELETE /api/_c/api-tokens/v1/{id}        revoke one (`pocketone logout` revokes its own)
//	POST   /api/_c/api-tokens/v1/revoke-all  revoke all of yours
//
// Extensions that accept API tokens (deploy, mcp, remotes) work with it through this package alone: they register
// their scopes with the [Issuer], read the token a request came with ([FromRequest]) and limit in-process requests
// ([WithScopes]). The OAuth server requires it.
package apitokensv1

import (
	"context"
	"slices"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Issuer is an app's API tokens. Its provider also serves the token API (extkit.RouteBinder) and authenticates
// requests that carry a token (setting [RequestKey]).
type Issuer interface {
	// Verify resolves a token secret to who it acts as and what it may do.
	Verify(ctx context.Context, secret string) (Grant, error)
	// RegisterScope adds a scope a token can be limited to (an extension's own, e.g. deploy's).
	RegisterScope(s Scope) error
	// Scopes lists the registered scopes.
	Scopes() []Scope
	// Reaches reports whether a token limited to scopes may call method on path.
	Reaches(scopes []string, method, path string) bool
}

// Grant is what a verified token allows.
type Grant struct {
	TokenID         string   `json:"tokenId"`
	Name            string   `json:"name,omitempty"` // the token's label
	OwnerCollection string   `json:"ownerCollection"`
	OwnerID         string   `json:"ownerId"`
	Scopes          []string `json:"scopes"`             // empty: the owner's full rights
	Audience        string   `json:"audience,omitempty"` // the resource it's bound to (RFC 8707), or ""
	ClientID        string   `json:"clientId,omitempty"` // the OAuth client it was issued to, or ""

	Owner *core.Record `json:"-"` // the owner's record (inside the app)
}

// HasScope reports whether the token may act with scope: unscoped tokens have every scope.
func (g *Grant) HasScope(scope string) bool {
	return len(g.Scopes) == 0 || slices.Contains(g.Scopes, scope)
}

// Scope is a named slice of an owner's rights that a token can be limited to. A token whose scopes are set
// authenticates only on requests its scopes match (Paths + Methods, or Match).
type Scope struct {
	// Name, e.g. "deploy" or "notes:write": lower-case letters, digits, ":" and "_", starting with a letter or digit.
	Name string `json:"name"`
	// Description is shown on the consent page and in the token screens.
	Description string `json:"description"`
	// Paths are URL path prefixes where a token with this scope authenticates as its owner.
	Paths []string `json:"paths,omitempty"`
	// Methods, when not empty, limits Paths to these HTTP methods (GET also covers HEAD). Empty: every method.
	Methods []string `json:"methods,omitempty"`
	// Match is an optional guard: a token with this scope also authenticates on any request it returns true for.
	// It runs before the owner is loaded, so it may look only at the request, not at e.Auth.
	Match func(e *core.RequestEvent) bool `json:"-"`
	// Collections, when not empty, limits which auth collections may hold a token with this scope.
	Collections []string `json:"collections,omitempty"`
}

// Infrastructure reports whether the scope only reaches PocketOne's own routes (contract APIs under /api/_c/): a
// client that needs one asks for it; consent pages and MCP don't offer it otherwise.
func (s Scope) Infrastructure() bool {
	if s.Match != nil || len(s.Paths) == 0 {
		return false
	}
	for _, p := range s.Paths {
		if !strings.HasPrefix(p, "/api/_c/") {
			return false
		}
	}
	return true
}

// OfflineAccess is the OAuth scope asking for a refresh token; it limits nothing.
const OfflineAccess = "offline_access"

// RequestKey is the request store key holding the *Grant of a request authenticated by an API token.
const RequestKey = "api-tokens.grant"

// FromRequest returns the API token a request was authenticated with, or nil (no auth, or a JWT).
func FromRequest(e *core.RequestEvent) *Grant {
	g, _ := e.Get(RequestKey).(*Grant)
	return g
}

type scopesKey struct{}

// WithScopes limits in-process requests made with ctx to these scopes, the way a token limited to them would be
// (ext/mcp's tool calls on a scoped connection). A nested WithScopes can only narrow it. An empty (or nil) list allows
// nothing but the token-wide paths, so call it only for a scoped connection.
func WithScopes(ctx context.Context, scopes []string) context.Context {
	next := slices.Clone(scopes)
	if prev, ok := ScopesFromContext(ctx); ok {
		next = slices.DeleteFunc(next, func(n string) bool { return !slices.Contains(prev, n) })
	}
	if next == nil {
		next = []string{}
	}
	return context.WithValue(ctx, scopesKey{}, next)
}

// ScopesFromContext returns the scopes set with WithScopes, and whether there are any limits.
func ScopesFromContext(ctx context.Context) ([]string, bool) {
	s, ok := ctx.Value(scopesKey{}).([]string)
	return s, ok
}

// Contract is api-tokens/v1.
var Contract = extkit.NewContract[Issuer]("api-tokens/v1", extkit.One)

// Prefix is where the token API is served: /api/_c/api-tokens/v1.
var Prefix = extkit.Routed(Contract)
