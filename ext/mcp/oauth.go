package mcp

import (
	"fmt"
	oauthv1 "github.com/pockethost/pocketone/ext/contracts/oauth/v1"
	tokensv1 "github.com/pockethost/pocketone/ext/contracts/tokens/v1"
	"net/url"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// OAuth signs MCP requests in with access tokens from the OAuth server in ext/tokens (API tokens from ext/tokens), the way Claude
// connectors and Claude Code expect: the client follows the 401 to the protected resource metadata, registers itself
// (CIMD or dynamic registration), signs the user in in a browser with PKCE, and gets a token bound to this endpoint
// (RFC 8707 audience). The token carries the account's rights, like a PocketBase auth token, or only the scopes the
// user ticked on the consent page; tool calls run as that account, limited to those scopes. Pasted PocketBase auth
// tokens work too, as on the REST API.
//
// It's the default [Authenticator] when the OAuth server is on on the app.
type OAuth struct {
	// Issuer overrides the authorization server listed in the protected resource metadata (default: the OAuth server's
	// own issuer, i.e. its Config.Issuer or the request's scheme and host).
	Issuer string
}

// Authenticate implements [Authenticator].
func (a OAuth) Authenticate(e *core.RequestEvent, resource string) (*Grant, error) {
	if e.Auth == nil {
		if strings.TrimSpace(e.Request.Header.Get("Authorization")) == "" {
			return nil, ErrNoCredentials
		}
		return nil, ErrInvalidToken
	}
	info := tokensv1.FromRequest(e)
	if info == nil || info.Audience == "" {
		// a PocketBase auth token, or an API token not bound to a resource: tool calls forward it unchanged, so
		// whatever limits it has on the REST API still apply
		return &Grant{Auth: e.Auth, Credential: e.Request.Header.Get("Authorization")}, nil
	}
	if !sameResource(info.Audience, resource) {
		return nil, fmt.Errorf("%w: the token was issued for %s", ErrInvalidToken, info.Audience)
	}
	// a token limited to some scopes limits the connection: tool calls are checked against them
	var scopes []string
	for _, sc := range info.Scopes {
		if sc != tokensv1.OfflineAccess {
			scopes = append(scopes, sc)
		}
	}
	return &Grant{Auth: info.Owner, ClientID: info.ClientID, Scopes: scopes}, nil
}

// AuthorizationServers implements [Authenticator].
func (a OAuth) AuthorizationServers(e *core.RequestEvent, s *Server) []string {
	if a.Issuer != "" {
		return []string{strings.TrimRight(a.Issuer, "/")}
	}
	if osrv, ok := oauthv1.Contract.Of(s.app); ok {
		return []string{osrv.Issuer(e)}
	}
	scheme := "http"
	if e.IsTLS() || strings.EqualFold(e.Request.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return []string{scheme + "://" + e.Request.Host}
}

// sameResource compares two resource URLs: scheme and host ignore case, a trailing slash doesn't matter.
func sameResource(a, b string) bool {
	ua, err1 := url.Parse(strings.TrimRight(a, "/"))
	ub, err2 := url.Parse(strings.TrimRight(b, "/"))
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.EqualFold(ua.Scheme, ub.Scheme) && strings.EqualFold(ua.Host, ub.Host) && ua.Path == ub.Path
}

// setupAuth runs on serve, after every extension has registered: it picks the authenticator and turns on dynamic
// client registration when OAuth signs MCP in.
func (s *Server) setupAuth() {
	osrv, hasOAuth := oauthv1.Contract.Of(s.app)
	_, hasTokens := tokensv1.Contract.Of(s.app)
	if s.cfg.Auth == nil {
		if hasOAuth && hasTokens {
			s.cfg.Auth = OAuth{}
		} else {
			s.cfg.Auth = PocketBaseTokenAuth{}
			s.app.Logger().Warn("mcp: the OAuth server (ext/tokens) is off; /mcp accepts pasted PocketBase auth tokens only, and clients can't sign in by themselves")
		}
	}
	if _, ok := s.cfg.Auth.(OAuth); ok && hasOAuth {
		osrv.EnableDynamicRegistration()
		// An agent's connection should stay signed in: MCP access tokens don't expire; revoke one to end it.
		osrv.SetAccessTTL(DefaultPath, 0)
	}
}
