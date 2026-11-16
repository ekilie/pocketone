// Package gatev1 is the gate/v1 contract: what admits requests on a multisite host, before an instance is served or
// forwarded (a firewall: peer allow lists, client IPs, rate limits). Gates run in order, every one must admit.
//
// A host-scope service contract: providers register on the host's scope (ProvideIn), not on an app.
package gatev1

import (
	"github.com/pockethost/pocketone/ext/extkit"
	"net/http"
	"net/netip"

	sourcev1 "github.com/pockethost/pocketone/ext/contracts/instance/source/v1"
)

// Gate admits requests. AllowPeer runs before routing and refuses with 403; Admit runs before an instance's request
// is served here or forwarded to another server, may answer it itself (ok=false), and otherwise returns a release
// function called when the response ends (long-lived responses hold it until they end).
type Gate interface {
	AllowPeer(r *http.Request) bool
	Admit(w http.ResponseWriter, r *http.Request, inst sourcev1.Instance) (release func(), ok bool)
}

// ClientIPResolver is an optional [Gate] extension: the request's real client (e.g. from CF-Connecting-IP when the
// peer is a trusted proxy). When a gate implements it, a request served here reaches the app with RemoteAddr set to
// that client, so PocketBase's logs, e.RealIP() and its own rate limits see the visitor instead of the proxy.
type ClientIPResolver interface {
	ClientIP(r *http.Request) netip.Addr
}

// Contract is gate/v1, on a host's scope.
var Contract = extkit.NewContract[Gate]("gate/v1", extkit.Many)
