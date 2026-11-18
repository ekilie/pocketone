// Package multisite is PocketOne's multi-instance host: the hostname directory, the request front (peer check,
// resolve, gates, then a runtime or the forward hook), listeners and TLS. What plugs into it are host-scope contracts
// (ext/contracts): instance.source/v1 (where instances come from), runtime/v1 (what runs them: ext/pocker
// as isolates, ext/docker in containers, one or several), gate/v1, tls/v1 and http.wrap/v1, provided by host
// extensions on the host's scope. The fleet (forwarding to other edges) is ext/edge, on top.
package multisite
