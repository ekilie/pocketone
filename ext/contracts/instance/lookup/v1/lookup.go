// Package lookupv1 is the instance.lookup/v1 contract: a multisite host asks for a hostname it doesn't know. A
// companion to instance.source/v1 for sources that don't hold every instance (a lazy mirror of a control plane): the
// host asks on every request it can't route, and the provider answers from its cache or fetches.
//
// A host-scope service contract; ext/mothership/client provides it beside instance.source/v1.
package lookupv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	sourcev1 "pocketone.dev/ext/contracts/instance/source/v1"
)

// Lookup answers what a hostname serves.
//
//contract:id instance.lookup/v1
//contract:kind service
type Lookup interface {
	// Lookup returns the instance host routes to. Found false (with a nil error) means no instance has the name; an
	// error means the answer isn't known (the control plane can't be reached), so the host answers 503, not 404.
	Lookup(ctx context.Context, host string) (Result, error)
}

// Result is a lookup's answer.
type Result struct {
	Found    bool
	Instance sourcev1.Instance
}
