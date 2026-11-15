package tokens

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
)

// Request store keys set when a request is authenticated by an API token. Plugins that don't import this package
// (remotes' whoami) read them by name.
const (
	RequestKeyInfo   = "tokens.info"   // *Info
	RequestKeyScopes = "tokens.scopes" // []string (empty: full rights)
	RequestKeyId     = "tokens.id"     // string, the token's record id
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
			// only this plugin sets scope headers: drop any the client sent, on every request
			stripScopeHeaders(e.Request.Header)
			secret := bearer(e)
			if !strings.HasPrefix(secret, Prefix) {
				return e.Next()
			}
			info, err := t.Verify(secret)
			if err != nil {
				// like an invalid JWT: the request goes on unauthenticated
				e.App.Logger().Debug("tokens: rejected API token", "error", err)
				return e.Next()
			}
			path := e.Request.URL.Path
			if isTokenMintingPath(path) {
				return e.ForbiddenError("API tokens can't be exchanged for other auth tokens.", nil)
			}
			if !audienceAllows(info.Audience, path) {
				return e.ForbiddenError("This API token is bound to another resource ("+info.Audience+").", nil)
			}
			if !t.registry.allows(info.Scopes, e) {
				return e.ForbiddenError("This API token's scopes ("+strings.Join(info.Scopes, ", ")+") don't allow this request.", nil)
			}
			e.Auth = info.Owner
			e.Set(RequestKeyInfo, info)
			e.Set(RequestKeyScopes, info.Scopes)
			e.Set(RequestKeyId, info.Token.Id)
			t.touch(info.Token, e.RealIP())
			return e.Next()
		},
	}
}

// scopeHeadersMiddleware sets `X-Token-Scope-<Name>: 1` for each scope a signed-in request holds, so collection
// rules can read them (`@request.headers.x_token_scope_notes_write = "1"`) and hooks can see them. A JWT or an
// unscoped token holds every registered scope; a scoped token only its own; an anonymous request none. It runs
// after PocketBase's JWT middleware and before anything builds the request info that rules read.
func (t *Tokens) scopeHeadersMiddleware() *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Id:       ScopeHeadersMiddlewareId,
		Priority: apis.DefaultLoadAuthTokenMiddlewarePriority + 1,
		Func: func(e *core.RequestEvent) error {
			if e.Auth != nil {
				held := t.heldScopes(e)
				for _, name := range held {
					e.Request.Header.Set(ScopeHeader(name), "1")
				}
			}
			return e.Next()
		},
	}
}

// heldScopes: the scopes a signed-in request holds (see scopeHeadersMiddleware).
func (t *Tokens) heldScopes(e *core.RequestEvent) []string {
	if i := FromRequest(e); i != nil && len(i.Scopes) > 0 {
		return i.Scopes
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
	for _, p := range AnyTokenPaths {
		if pathUnder(path, p) {
			return true
		}
	}
	u, err := url.Parse(audience)
	if err != nil {
		return false
	}
	ap := strings.TrimSuffix(u.Path, "/")
	return ap == "" || pathUnder(path, ap)
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
// scope; a scoped token only its own.
func HasScope(e *core.RequestEvent, scope string) bool {
	if e.Auth == nil {
		return false
	}
	if i := FromRequest(e); i != nil {
		return i.HasScope(scope)
	}
	return true
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
