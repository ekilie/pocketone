package apitokens

import (
	"context"
	"net/http"
	"net/url"
	apitokensv1 "pocketone.dev/ext/contracts/api-tokens/v1"
	"slices"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
)

// Request store keys set when a request is authenticated by an API token. Extensions that don't import this package
// (remotes' whoami) read them by name.
const (
	RequestKeyInfo   = "api-tokens.info"   // *Info
	RequestKeyScopes = "api-tokens.scopes" // []string (empty: full rights)
	RequestKeyId     = "api-tokens.id"     // string, the token's record id
)

// MiddlewareId is the id of the bearer middleware bound on every route.
const MiddlewareId = "pbTokensAuth"

// ScopeHeadersMiddlewareId is the id of the middleware that sets the X-Token-Scope-* headers, right after
// PocketBase loads a JWT.
const ScopeHeadersMiddlewareId = "pbTokensScopeHeaders"

// middleware resolves `Authorization: Bearer pbt_...` to the token's owner before PocketBase's own JWT middleware
// runs (which then sees e.Auth set and does nothing).
func (t *Tokens) middleware() *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Id:       MiddlewareId,
		Priority: apis.DefaultLoadAuthTokenMiddlewarePriority - 1,
		Func: func(e *core.RequestEvent) error {
			// only this extension sets scope headers: drop any the client sent, on every request
			stripScopeHeaders(e.Request.Header)
			// an in-process request made for a scoped connection (ext/mcp): checked like a scoped token's request
			if limit, ok := ScopesFromContext(e.Request.Context()); ok && !t.registry.allowsLimit(limit, e) {
				return e.ForbiddenError("This connection's scopes ("+strings.Join(limit, ", ")+") don't allow this request.", nil)
			}
			secret := bearer(e)
			if t.prefixOf(secret) == "" {
				return e.Next()
			}
			info, err := t.Verify(secret)
			if err != nil {
				// like an invalid JWT: no auth is loaded, so PocketBase's pbRejectUnknownToken answers 401
				e.App.Logger().Debug("api-tokens: rejected API token", "error", err)
				return e.Next()
			}
			path := e.Request.URL.Path
			if isTokenMintingPath(path) {
				return e.ForbiddenError("API tokens can't be exchanged for other auth tokens.", nil)
			}
			if !audienceAllows(info.Audience, path) {
				return e.ForbiddenError("This API token is bound to another resource ("+info.Audience+").", nil)
			}
			// a token bound to a resource authenticates there whatever its scopes: they limit what the resource does
			// on its behalf (ext/mcp passes them on with WithScopes), not the resource's own endpoint
			if !boundHere(info.Audience, path) && !t.registry.allows(info.Scopes, e) {
				return e.ForbiddenError("This API token's scopes ("+strings.Join(info.Scopes, ", ")+") don't allow this request.", nil)
			}
			e.Auth = info.Owner
			e.Set(RequestKeyInfo, info)
			g := info.grant()
			e.Set(apitokensv1.RequestKey, &g)
			e.Set(RequestKeyScopes, info.Scopes)
			e.Set(RequestKeyId, info.Token.Id)
			t.touch(info.Token, e.RealIP())
			return e.Next()
		},
	}
}

// scopeHeadersMiddleware sets `X-Token-Scope-<Name>: 1` for each scope a signed-in request holds, so collection
// rules can read them (`@request.headers.x_token_scope_notes_write = "1"`) and hooks can see them. A JWT or an
// unscoped token holds every registered scope; a scoped token only its own; an anonymous request none. A scope held
// for some values only ("notes=a", "notes=b") gets those values instead of "1" ("a,b"), so `= "1"` stays "every
// value". It runs after PocketBase's JWT middleware and before anything builds the request info that rules read.
func (t *Tokens) scopeHeadersMiddleware() *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Id:       ScopeHeadersMiddlewareId,
		Priority: apis.DefaultLoadAuthTokenMiddlewarePriority + 1,
		Func: func(e *core.RequestEvent) error {
			if e.Auth != nil {
				held := t.heldScopes(e)
				for _, s := range held {
					name, _ := apitokensv1.SplitScope(s)
					v := "1"
					if vals, all := apitokensv1.ValuesOf(held, name); !all {
						v = strings.Join(vals, ",")
					}
					e.Request.Header.Set(ScopeHeader(name), v)
				}
			}
			return e.Next()
		},
	}
}

// heldScopes: the scopes a signed-in request holds (see scopeHeadersMiddleware), narrowed to the context's scopes
// when the request was made for a scoped connection.
func (t *Tokens) heldScopes(e *core.RequestEvent) []string {
	held := t.ownScopes(e)
	if limit, ok := requestLimit(e); ok {
		held = apitokensv1.IntersectScopes(held, limit)
	}
	return held
}

func (t *Tokens) ownScopes(e *core.RequestEvent) []string {
	if i := FromRequest(e); i != nil && len(i.Scopes) > 0 {
		return slices.Clone(i.Scopes)
	}
	all := t.registry.list()
	out := make([]string, 0, len(all))
	for _, s := range all {
		if len(s.Collections) > 0 && !slices.Contains(s.Collections, e.Auth.Collection().Name) {
			continue // this account could never hold the scope
		}
		out = append(out, s.Name)
	}
	return out
}

func stripScopeHeaders(h http.Header) {
	for k := range h {
		if len(k) >= len(HeaderPrefix) && strings.EqualFold(k[:len(HeaderPrefix)], HeaderPrefix) {
			delete(h, k)
		}
	}
}

// audienceAllows: a token bound to a resource URL works only under that URL's path (and the any-token paths).
func audienceAllows(audience, path string) bool {
	if audience == "" {
		return true
	}
	if anyTokenPath(path) {
		return true
	}
	u, err := url.Parse(audience)
	if err != nil {
		return false
	}
	ap := strings.TrimSuffix(u.Path, "/")
	return ap == "" || pathUnder(path, ap)
}

// boundHere: the token is bound to a resource with a path (e.g. https://host/mcp) and the request is under it. A
// resource without a path (the whole host) doesn't count: there the token's scopes still decide.
func boundHere(audience, path string) bool {
	if audience == "" {
		return false
	}
	u, err := url.Parse(audience)
	if err != nil {
		return false
	}
	ap := strings.TrimSuffix(u.Path, "/")
	return ap != "" && pathUnder(path, ap)
}

func bearer(e *core.RequestEvent) string {
	h := e.Request.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return strings.TrimSpace(h)
}

// isTokenMintingPath: routes that would turn a revocable API token into a JWT (which revoking can't reach).
func isTokenMintingPath(path string) bool {
	if !strings.HasPrefix(path, "/api/collections/") {
		return false
	}
	rest := strings.TrimPrefix(path, "/api/collections/")
	_, action, ok := strings.Cut(rest, "/")
	return ok && (action == "auth-refresh" || strings.HasPrefix(action, "impersonate/"))
}

// FromRequest returns the API token a request was authenticated with, or nil (no auth, or a JWT).
func FromRequest(e *core.RequestEvent) *Info {
	i, _ := e.Get(RequestKeyInfo).(*Info)
	return i
}

// HasScope reports whether an authenticated request may act with scope: JWT auth and unscoped tokens have every
// scope; a scoped token only its own; a request made for a scoped connection (WithScopes) only the connection's. A
// bare name is held by a token holding it for some values only; to check one value use [HasScopeFor].
func HasScope(e *core.RequestEvent, scope string) bool {
	if e.Auth == nil {
		return false
	}
	if limit, ok := requestLimit(e); ok && (len(limit) == 0 || !apitokensv1.HoldsScope(limit, scope)) {
		return false
	}
	if i := FromRequest(e); i != nil {
		return i.HasScope(scope)
	}
	return true
}

// HasScopeFor reports whether an authenticated request may act with scope on value: as [HasScope], where a token
// holding the scope for some values only must hold this one ("instances=abc123").
func HasScopeFor(e *core.RequestEvent, scope, value string) bool {
	return HasScope(e, apitokensv1.JoinScope(scope, value))
}

// ValuesFor lists the values an authenticated request holds scope for; all is true when it may use every value
// (JWT auth, an unscoped token, or the bare scope). Not signed in: none.
func ValuesFor(e *core.RequestEvent, scope string) (values []string, all bool) {
	if e.Auth == nil {
		return nil, false
	}
	var held []string // nil: full rights
	if i := FromRequest(e); i != nil && len(i.Scopes) > 0 {
		held = i.Scopes
	}
	if limit, ok := requestLimit(e); ok {
		if held == nil {
			held = limit
		} else {
			held = apitokensv1.IntersectScopes(held, limit)
		}
		if len(held) == 0 {
			return nil, false
		}
	}
	return apitokensv1.ValuesOf(held, scope)
}

// RequireScope requires an authenticated request (JWT or API token) whose token, if any, has scope. Combine with
// apis.RequireSuperuserAuth() and friends for who may call the route.
func RequireScope(scope string) *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Id: "pbTokensRequireScope_" + scope,
		Func: func(e *core.RequestEvent) error {
			if e.Auth == nil {
				return e.UnauthorizedError("The request requires valid authorization.", nil)
			}
			if !HasScope(e, scope) {
				return e.ForbiddenError("This API token lacks the "+scope+" scope.", nil)
			}
			return e.Next()
		},
	}
}

// WithScopes is api-tokens/v1's [apitokensv1.WithScopes]: it limits in-process requests made with ctx to these scopes, the
// way a token limited to them would be; the tokens middleware refuses requests the scopes don't match (403).
func WithScopes(ctx context.Context, scopes []string) context.Context {
	return apitokensv1.WithScopes(ctx, scopes)
}

// ScopesFromContext returns the scopes set with WithScopes, and whether there are any limits.
func ScopesFromContext(ctx context.Context) ([]string, bool) {
	return apitokensv1.ScopesFromContext(ctx)
}

func requestLimit(e *core.RequestEvent) ([]string, bool) {
	if e.Request == nil {
		return nil, false
	}
	return ScopesFromContext(e.Request.Context())
}
