package mcp

import (
	"fmt"
	"net/url"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	oauthserver "github.com/pockethost/pocketone/ext/oauth-server"
	"github.com/pockethost/pocketone/ext/tokens"
)

// OAuth signs MCP requests in with access tokens from ext/oauth-server (API tokens from ext/tokens), the way Claude
// connectors and Claude Code expect: the client follows the 401 to the protected resource metadata, registers itself
// (CIMD or dynamic registration), signs the user in in a browser with PKCE, and gets a token bound to this endpoint
// (RFC 8707 audience). The token carries the account's rights, like a PocketBase auth token, or only the scopes the
// user ticked on the consent page; tool calls run as that account, limited to those scopes. Pasted PocketBase auth
// tokens work too, as on the REST API.
//
// It's the default [Authenticator] when oauth-server is registered on the app.
type OAuth struct {
	// Issuer overrides the authorization server listed in the protected resource metadata (default: oauth-server's
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
	info := tokens.FromRequest(e)
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
		if sc != oauthserver.OfflineAccess {
			scopes = append(scopes, sc)
		}
	}
	return &Grant{Auth: info.Owner, ClientID: info.Token.GetString("client_id"), Scopes: scopes}, nil
}

// AuthorizationServers implements [Authenticator].
func (a OAuth) AuthorizationServers(e *core.RequestEvent, s *Server) []string {
	if a.Issuer != "" {
		return []string{strings.TrimRight(a.Issuer, "/")}
	}
	if osrv := oauthserver.For(s.app); osrv != nil {
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

// setupAuth runs on serve, after every plugin has registered: it picks the authenticator and turns on dynamic
// client registration when OAuth signs MCP in.
func (s *Server) setupAuth() {
	osrv := oauthserver.For(s.app)
	if s.cfg.Auth == nil {
		if osrv != nil && tokens.For(s.app) != nil {
			s.cfg.Auth = OAuth{}
		} else {
			s.cfg.Auth = PocketBaseTokenAuth{}
			s.app.Logger().Warn("mcp: ext/oauth-server isn't registered; /mcp accepts pasted PocketBase auth tokens only, and clients can't sign in by themselves")
		}
	}
	if _, ok := s.cfg.Auth.(OAuth); ok && osrv != nil {
		osrv.EnableDynamicRegistration()
		// An agent's connection should stay signed in: MCP access tokens don't expire; revoke one to end it.
		osrv.SetAccessTTL(s.cfg.Path, 0)
	}
}
