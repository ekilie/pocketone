package extkit

import (
	"context"
	"net/http"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/router"
)

// A routed contract brings its own HTTP API instead of generated operations, served under /api/_c/<id>/. It keeps the
// rest of a contract: the lookup, defaults and providers: choices, host-providability, and whoami's contract list.
// Use it when a protocol fixes the API's shape (OAuth) or an existing API moves under a contract as it is (deploy/v1's
// file API, tokens/v1's token API).
//
// The provider serves the API one of three ways, checked in this order:
//
//   - [RouteBinder]: it binds its own routes on a router group at the prefix (path parameters, middleware);
//
//   - [EventServer]: it serves PocketBase request events (inside an app, with PocketBase's error responses);
//
//   - http.Handler: it serves every request under the prefix (the only way a host can serve it).
//
//     var Contract = extkit.NewContract[http.Handler]("deploy/v1", extkit.One).HostProvidable()
//     var Prefix = extkit.Routed(Contract)
type routed[T any] struct{ c *Contract[T] }

// Routed registers c as a routed contract and returns its path prefix ("/api/_c/<id>").
func Routed[T any](c *Contract[T]) string {
	wiresMu.Lock()
	wires = append(wires, routed[T]{c})
	wiresMu.Unlock()
	return opPath(c.id, "")
}

func (r routed[T]) id() ContractID        { return r.c.id }
func (r routed[T]) info() WireInfo        { return WireInfo{ID: r.c.id} } // its own API shape: no ops to list
func (r routed[T]) scopes() []WireScope   { return nil }                  // its provider registers its own token scopes
func (r routed[T]) tools(core.App) []Tool { return nil }                  // its API's shape is its provider's: no generated tools
func (r routed[T]) provided(app core.App) bool {
	_, ok := r.c.Of(app)
	return ok
}

// RouteBinder is a routed contract's provider that binds its own routes on a router group at the contract's prefix.
type RouteBinder interface {
	BindRoutes(g *router.RouterGroup[*core.RequestEvent])
}

// EventServer is a routed contract's provider that serves PocketBase request events directly (inside an app), so its
// errors get PocketBase's usual responses.
type EventServer interface {
	ServeEvent(e *core.RequestEvent) error
}

// OwnsAuthorization is an optional interface for a routed contract's provider that reads the Authorization header
// itself (an OAuth client's "Basic" credentials, say). Its routes skip PocketBase's pbRejectUnknownToken, which
// answers 401 to any Authorization header that isn't a known PocketBase token; the provider rejects bad ones itself.
// It is checked once, on the provider found when the routes are mounted, so a decorator (Contract.Wrap) must forward
// it. A RouteBinder provider binds its own routes and unbinds the middleware there instead.
type OwnsAuthorization interface {
	OwnsAuthorization() bool
}

type requestEventKey struct{}

// RequestEvent is the PocketBase request a routed contract's http.Handler is serving inside an app (nil when a host
// serves it), for the auth record and API token on it.
func RequestEvent(r *http.Request) *core.RequestEvent {
	e, _ := r.Context().Value(requestEventKey{}).(*core.RequestEvent)
	return e
}

// Mount serves the routed contract inside app.
func (r routed[T]) Mount(app core.App, se *core.ServeEvent) {
	prefix := opPath(r.c.id, "")
	impl, ok := r.c.Of(app)
	if !ok {
		return
	}
	if rb, isBinder := any(impl).(RouteBinder); isBinder {
		rb.BindRoutes(se.Router.Group(prefix))
		return
	}
	h := func(e *core.RequestEvent) error {
		impl, ok := r.c.Of(app)
		if !ok {
			return e.NotFoundError("this app doesn't provide "+string(r.c.id), nil)
		}
		if ev, isEvent := any(impl).(EventServer); isEvent {
			return ev.ServeEvent(e) // PocketBase's own error responses
		}
		hh, isHandler := any(impl).(http.Handler)
		if !isHandler {
			return e.NotFoundError("", nil)
		}
		req := e.Request.WithContext(context.WithValue(e.Request.Context(), requestEventKey{}, e))
		hh.ServeHTTP(e.Response, req)
		return nil
	}
	unbind := []string{"pbBodyLimit"} // the API sets its own limits
	if oa, owns := any(impl).(OwnsAuthorization); owns && oa.OwnsAuthorization() {
		unbind = append(unbind, apis.DefaultRejectUnknownTokenMiddlewareId)
	}
	// explicit methods: a method-less pattern would conflict with a static "GET /{path...}" route
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		se.Router.Route(m, prefix, h).Unbind(unbind...)
		se.Router.Route(m, prefix+"/{path...}", h).Unbind(unbind...)
	}
}

// HostRoutes answers a routed contract for a host's instances, in front of the runtime (on a multisite host: one of
// its InstanceRoutes, after the gates): requests under
// /api/_c/<id>/ on an instance's hostname go to the host's provider for that instance (an http.Handler); everything
// else goes to next.
func HostRoutes[T any](host *Scope, c *Contract[T], instance func(*http.Request) (string, bool), next http.Handler) http.Handler {
	prefix := opPath(c.id, "")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Path != prefix && !strings.HasPrefix(r.URL.Path, prefix+"/")) || !host.HostProvides(c.id) {
			next.ServeHTTP(w, r)
			return
		}
		inst, ok := instance(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		impl, _ := c.ForInstance(host, inst)
		h, ok := any(impl).(http.Handler)
		if !ok {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}
