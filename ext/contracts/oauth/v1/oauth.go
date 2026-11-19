// Package oauthv1 is the oauth/v1 contract: an OAuth 2.1 authorization server for the app (authorization code with
// PKCE, device code, refresh, revocation, dynamic and metadata-document client registration), so the CLI and MCP
// clients like Claude can sign in. ext/api-tokens provides it.
//
// A routed contract: the OAuth specs fix its shape, so its provider serves its own routes under /api/_c/oauth/v1/:
//
//	GET|POST /api/_c/oauth/v1/authorize         sign in and consent (browser)
//	POST     /api/_c/oauth/v1/token             codes and refresh tokens for access tokens
//	POST     /api/_c/oauth/v1/device            start a device-code sign-in
//	GET|POST /api/_c/oauth/v1/device/verify     enter the device code (browser)
//	POST     /api/_c/oauth/v1/revoke            revoke a token
//	POST     /api/_c/oauth/v1/register          dynamic client registration (when on)
//	GET|POST /api/_c/oauth/v1/provider/callback sign-in through an OAuth2 provider (Google, Apple, ...)
//
// Its metadata stays where RFC 8414 puts it: GET /.well-known/oauth-authorization-server.
package oauthv1

import (
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"pocketone.dev/ext/extkit"
)

// Server is the app's authorization server, as other extensions (ext/mcp) use it. Its provider serves the routes
// above (extkit.EventServer).
type Server interface {
	// Issuer is the server's issuer URL, as seen by the request.
	Issuer(e *core.RequestEvent) string
	// EnableDynamicRegistration turns on POST /register (MCP clients register themselves).
	EnableDynamicRegistration()
	// SetAccessTTL sets how long access tokens for the resource at resourcePath live (0: until revoked).
	SetAccessTTL(resourcePath string, ttl time.Duration)
}

// MetadataPath is where the server's metadata lives (RFC 8414 fixes it at the root): a client that finds oauth/v1 in
// whoami's contracts reads it here.
const MetadataPath = "/.well-known/oauth-authorization-server"

// Contract is oauth/v1.
var Contract = extkit.NewContract[Server]("oauth/v1", extkit.One)

// Prefix is where oauth/v1 is served: /api/_c/oauth/v1.
var Prefix = extkit.Routed(Contract)
