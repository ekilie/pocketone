// Package tlsv1 is the tls/v1 contract: the TLS of a multisite host's main listener (automatic certificates). With
// no provider the host uses the certificate files in its `server` section, or plain HTTP.
//
// A host-scope service contract: providers register on the host's scope (ProvideIn), not on an app.
package tlsv1

import (
	"context"
	"crypto/tls"
	"net/http"
	"pocketone.dev/ext/extkit"
)

// Provider supplies the main listener's TLS. served reports whether a hostname belongs to an instance, for providers
// that must not issue certificates for anything else. httpHandler, if not nil, wraps the plain-HTTP listener's
// handler (tls.http_listen), e.g. to answer ACME challenges. A nil config means "not this time" (the host falls back
// to its certificate files or plain HTTP).
type Provider interface {
	TLS(ctx context.Context, served func(host string) bool) (cfg *tls.Config, httpHandler func(http.Handler) http.Handler, err error)
}

// Contract is tls/v1, on a host's scope.
var Contract = extkit.NewContract[Provider]("tls/v1", extkit.One)
