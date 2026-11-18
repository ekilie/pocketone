// Package runtimev1 is the runtime/v1 contract: what runs a multisite host's instances (ext/pocker as isolates,
// ext/docker in containers). A host may have several, in preference order: each instance runs on the one it names
// (Instance.Runtime), else the first that serves it. `providers: { runtime/v1: [pocker, docker] }` picks and orders
// them.
//
// A host-scope service contract: providers register on the host's scope (ProvideIn), not on an app.
package runtimev1

import (
	"context"
	"github.com/pockethost/pocketone/ext/extkit"
	"net/http"

	sourcev1 "github.com/pockethost/pocketone/ext/contracts/instance/source/v1"
)

// Runtime runs instances on this host.
type Runtime interface {
	// Name identifies the runtime ("pocker", "docker"); an instance may ask for one in Instance.Runtime.
	Name() string
	// Serves reports whether this runtime can run inst (e.g. its PocketBase version).
	Serves(inst sourcev1.Instance) bool
	// ServeInstance serves a request for inst, starting it on demand. The host has already resolved the instance,
	// admitted the request (gates) and decided it runs here.
	ServeInstance(w http.ResponseWriter, r *http.Request, inst sourcev1.Instance)
	// Ready reports whether the runtime can take requests.
	Ready() bool
}

// Optional runtime hooks: a runtime implements the ones it needs, and the host checks each with a type assertion.

// Prechecker runs before the gates admit a request for a local instance, e.g. to answer 503 for an instance under
// maintenance, without counting it against rate limits. false = answered.
type Prechecker interface {
	Precheck(w http.ResponseWriter, r *http.Request, inst sourcev1.Instance) bool
}

// Syncer hears directory changes (stop removed instances, restart changed ones).
type Syncer interface {
	Sync(c sourcev1.Change, prev *sourcev1.Instance)
}

// Starter runs once before the host listens, e.g. to adopt containers that kept running across a restart. An error
// stops the host from starting.
type Starter interface {
	Start(ctx context.Context) error
}

// Shutdowner is called when the host stops.
type Shutdowner interface {
	Shutdown()
}

// Statser reports how many instances it has running, for the health route.
type Statser interface {
	RunningCount() int
}

// Contract is runtime/v1, on a host's scope.
var Contract = extkit.NewContract[Runtime]("runtime/v1", extkit.Many)
