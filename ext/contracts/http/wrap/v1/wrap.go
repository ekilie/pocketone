// Package wrapv1 is the http.wrap/v1 contract: middleware around everything a multisite host serves (a
// Server-Timing header, request logging). Wrappers nest in provider order: the first is the outermost.
//
// A host-scope service contract: providers register on the host's scope (ProvideIn), not on an app.
package wrapv1

import (
	"net/http"

	"github.com/pockethost/pocketone/ext/extkit"
)

// Wrapper wraps the host's handler.
type Wrapper interface {
	Wrap(next http.Handler) http.Handler
}

// Contract is http.wrap/v1, on a host's scope.
var Contract = extkit.NewContract[Wrapper]("http.wrap/v1", extkit.Many)
