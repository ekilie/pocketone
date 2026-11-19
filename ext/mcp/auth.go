package mcp

import (
	"errors"
	"fmt"
	"net/http"
	apitokensv1 "pocketone.dev/ext/contracts/api-tokens/v1"
	mcpv1 "pocketone.dev/ext/contracts/mcp/v1"
	"slices"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Grant is who a valid credential signs in, and how tool calls forward it.
type Grant struct {
	// Auth is the signed-in record; tool calls run as it.
	Auth *core.Record
	// ClientID of the OAuth client, when the credential came from one.
	ClientID string
	// Credential is the Authorization value tool calls forward to the REST API unchanged (the caller's own
	// PocketBase token). Empty: each MCP request makes a PocketBase token for Auth, as ext/pages' ctx.pb() does
	// (an OAuth token bound to /mcp isn't accepted on the REST API).
	Credential string
	// Scopes, when not empty, limit the connection like an API token limited to them (ext/api-tokens): tool calls
	// run with [apitokensv1.WithScopes], so requests the scopes don't allow are refused, and tools/list hides tools
	// none of them reaches.
	Scopes []string
}

var (
	// ErrNoCredentials means the request carried none: the client gets a 401 that points it at sign-in.
	ErrNoCredentials = errors.New("no credentials")
	// ErrInvalidToken means the credential is unknown, expired, revoked or for another audience.
	ErrInvalidToken = errors.New("invalid token")
)

// Authenticator signs MCP requests in: [OAuth] (the OAuth server in ext/api-tokens), or [PocketBaseTokenAuth] when the
// app has no OAuth server.
type Authenticator interface {
	// Authenticate checks the request's credentials for resource (the endpoint's canonical URL, which an OAuth token
	// must have been issued for). Return [ErrNoCredentials] or [ErrInvalidToken] (possibly wrapped) to send a 401.
	Authenticate(e *core.RequestEvent, resource string) (*Grant, error)

	// AuthorizationServers are the issuers listed in the protected resource metadata, primary first (Claude uses only
	// the first).
	AuthorizationServers(e *core.RequestEvent, s *Server) []string
}

// PocketBaseTokenAuth accepts ordinary PocketBase auth tokens (Authorization: Bearer <token>) for records of the
// listed collections (default: any auth collection). Tool calls forward the same token.
//
// It's the fallback for apps without the OAuth server, and handy in tests. Clients can't sign in by themselves with it.
type PocketBaseTokenAuth struct {
	Collections []string
}

// Authenticate implements [Authenticator].
func (a PocketBaseTokenAuth) Authenticate(e *core.RequestEvent, _ string) (*Grant, error) {
	if e.Auth == nil {
		if strings.TrimSpace(e.Request.Header.Get("Authorization")) == "" {
			return nil, ErrNoCredentials
		}
		return nil, ErrInvalidToken
	}
	if len(a.Collections) > 0 && !slices.Contains(a.Collections, e.Auth.Collection().Name) {
		return nil, fmt.Errorf("%w: %s records can't use MCP", ErrInvalidToken, e.Auth.Collection().Name)
	}
	return &Grant{Auth: e.Auth, Credential: e.Request.Header.Get("Authorization")}, nil
}

// AuthorizationServers implements [Authenticator]: the app itself, where ext/api-tokens' OAuth server answers.
func (a PocketBaseTokenAuth) AuthorizationServers(e *core.RequestEvent, s *Server) []string {
	return []string{s.publicOrigin(e)}
}

// MetadataURL is where this endpoint's protected resource metadata (RFC 9728) is served.
func (s *Server) MetadataURL(e *core.RequestEvent) string {
	return s.publicOrigin(e) + mcpv1.MetadataPath
}

// ProtectedResourceMetadata is the RFC 9728 document for this endpoint.
func (s *Server) ProtectedResourceMetadata(e *core.RequestEvent) map[string]any {
	name := s.app.Settings().Meta.AppName
	if name == "" {
		name = "PocketBase"
	}
	return map[string]any{
		"resource":                 s.ResourceURL(e),
		"authorization_servers":    s.cfg.Auth.AuthorizationServers(e, s),
		"bearer_methods_supported": []string{"header"},
		"resource_name":            name + " MCP",
		"scopes_supported":         s.offeredScopes(),
	}
}

// offeredScopes are the app's ext/api-tokens scopes a connection can be limited to (infrastructure scopes, which no
// tool reaches, aren't offered).
func (s *Server) offeredScopes() []string {
	out := []string{}
	if tk, ok := apitokensv1.Contract.Of(s.app); ok {
		for _, sc := range tk.Scopes() {
			if !sc.Infrastructure() {
				out = append(out, sc.Name)
			}
		}
	}
	return out
}

// challenge answers 401 with the WWW-Authenticate header MCP clients (Claude) need to find the sign-in.
func (s *Server) challenge(e *core.RequestEvent, err error) error {
	v := fmt.Sprintf(`Bearer resource_metadata=%q`, s.MetadataURL(e))
	msg := "Sign-in required."
	if err != nil && !errors.Is(err, ErrNoCredentials) {
		v += `, error="invalid_token"`
		msg = "The access token is invalid or expired."
	}
	e.Response.Header().Set("WWW-Authenticate", v)
	return e.JSON(http.StatusUnauthorized, map[string]any{"error": "unauthorized", "message": msg})
}
