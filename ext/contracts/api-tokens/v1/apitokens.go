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
	"errors"
	"slices"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"pocketone.dev/ext/extkit"
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

// Errors [Issuer.Verify] returns for a token that isn't valid; any other error is the provider failing.
var (
	ErrInvalid = errors.New("invalid API token") // unknown, malformed or revoked
	ErrExpired = errors.New("expired API token")
)

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

// HasScope reports whether the token may act with scope: unscoped tokens have every scope. A bare name ("instances")
// is held by "instances" and by any "instances=<value>"; "instances=abc" is held by "instances" or "instances=abc".
func (g *Grant) HasScope(scope string) bool { return HoldsScope(g.Scopes, scope) }

// HasScopeFor reports whether the token may act with scope on value: it holds the scope with no value (every value),
// or holds "scope=value". Unscoped tokens may.
func (g *Grant) HasScopeFor(scope, value string) bool { return HoldsScope(g.Scopes, JoinScope(scope, value)) }

// ScopeValues lists the values the token holds scope for. all is true when it holds the scope for every value (an
// unscoped token, or the bare name); otherwise values may be empty (it doesn't hold the scope).
func (g *Grant) ScopeValues(scope string) (values []string, all bool) { return ValuesOf(g.Scopes, scope) }

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
	// Values, when set, lets a token hold the scope for some values only ("instances=abc123"), e.g. record ids. The
	// bare name still means every value. Paths and Match don't see values: whoever serves the scope's requests
	// checks them ([Grant.HasScopeFor], [Grant.ScopeValues]).
	Values *ScopeValues `json:"values,omitempty"`
}

// ScopeValues describes what a scope's values are.
type ScopeValues struct {
	// Description says what a value is (e.g. "an instance id"); token screens show it.
	Description string `json:"description"`
	// Check, if set, vets a value when a token is created for owner (e.g. that owner owns that record); an error
	// refuses the token.
	Check func(owner *core.Record, value string) error `json:"-"`
}

// ScopeValueSep joins a scope name and a value in a token's scope list: "instances=abc123".
const ScopeValueSep = "="

// SplitScope splits a token scope into its name and value ("" when it has none).
func SplitScope(scope string) (name, value string) {
	name, value, _ = strings.Cut(scope, ScopeValueSep)
	return name, value
}

// JoinScope is "name=value", or name when value is "".
func JoinScope(name, value string) string {
	if value == "" {
		return name
	}
	return name + ScopeValueSep + value
}

// HoldsScope reports whether a token with these scopes may act with scope (see [Grant.HasScope]). No scopes: every
// scope.
func HoldsScope(scopes []string, scope string) bool {
	if len(scopes) == 0 {
		return true
	}
	return holds(scopes, scope)
}

// holds is HoldsScope for a non-empty list (an empty one holds nothing).
func holds(scopes []string, scope string) bool {
	name, value := SplitScope(scope)
	for _, s := range scopes {
		n, v := SplitScope(s)
		if n == name && (v == "" || value == "" || v == value) {
			return true
		}
	}
	return false
}

// CoversScope reports whether a token with these scopes may hand out scope to a token it creates: "instances" covers
// "instances" and "instances=abc"; "instances=abc" covers only "instances=abc". No scopes: everything.
func CoversScope(scopes []string, scope string) bool {
	if len(scopes) == 0 {
		return true
	}
	name, value := SplitScope(scope)
	for _, s := range scopes {
		n, v := SplitScope(s)
		if n == name && (v == "" || v == value) {
			return true
		}
	}
	return false
}

// ValuesOf is [Grant.ScopeValues] for a scope list. No scopes: every value.
func ValuesOf(scopes []string, scope string) (values []string, all bool) {
	if len(scopes) == 0 {
		return nil, true
	}
	for _, s := range scopes {
		n, v := SplitScope(s)
		if n != scope {
			continue
		}
		if v == "" {
			return nil, true
		}
		if !slices.Contains(values, v) {
			values = append(values, v)
		}
	}
	return values, false
}

// IntersectScopes is what both lists allow (both non-empty: an empty list here means nothing, not full rights):
// "instances" ∩ "instances=abc" is "instances=abc". The result is sorted.
func IntersectScopes(a, b []string) []string {
	out := []string{}
	add := func(s string) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	for _, s := range b {
		name, value := SplitScope(s)
		if CoversScope(a, s) && len(a) > 0 {
			add(s)
			continue
		}
		if value == "" { // b holds every value: keep a's values of it
			for _, x := range a {
				if n, _ := SplitScope(x); n == name {
					add(x)
				}
			}
		}
	}
	slices.Sort(out)
	return out
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
		next = IntersectScopes(prev, next)
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
