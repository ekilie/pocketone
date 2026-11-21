// Package resolvev1 is the instance.resolve/v1 contract: on a control plane (a mothership), what a hostname serves
// for a given edge, built from the control plane's own records, and which records each answer was built from. The
// mothership's watch server (instance.watch/v1) asks it on every watch and asks again after a write to any record an
// answer depends on, so it only needs to be right, never to track changes itself.
//
// The default provider is ext/mothership's own schema (`edges`, `instances`). A control plane with its own
// collections provides its own; one that only adds to the default's answers wraps it (Contract.Wrap).
package resolvev1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	sourcev1 "pocketone.dev/ext/contracts/instance/source/v1"
	watchv1 "pocketone.dev/ext/contracts/instance/watch/v1"
)

// Resolver builds answers from a control plane's records. Every answer lists its Deps: the records it read (see
// [Record], [Collection]), the hostnames whose claims decided it ([Host]) and any keys of its own ([Key]). After a
// write, the server re-asks every answer whose deps the written record touches: its own record and collection, and
// whatever [Resolver.Touches] reports.
//
//contract:id instance.resolve/v1
//contract:kind service
type Resolver interface {
	// Resolve answers what Host serves for Edge (an edge id). Not found is an answer (Instance nil), with deps too
	// (at least Host(host)), so the record that later claims the name re-resolves it.
	Resolve(ctx context.Context, in Query) (Resolution, error)

	// Standing lists the instances an edge must hold whatever its traffic (work addressed to it: a copy to make,
	// scheduled calls to send), with their deps. Nil is fine: most edges hold only what they serve.
	Standing(ctx context.Context, edge string) (Standing, error)

	// Edges lists every edge, with their deps.
	Edges(ctx context.Context) (EdgeList, error)

	// Touches lists the deps a write of rec touches besides its own record and collection: Host(name) for each
	// hostname it claims or claimed (compare rec.Original(): a new record's is blank, a deleted one's is rec), so
	// the watches on a name it adds, changes or drops resolve again; and Key(...) deps of the resolver's own. Report
	// only what the write changed: a record written often (an edge's heartbeat) mustn't re-resolve every answer
	// that merely looked it up.
	Touches(ctx context.Context, rec *core.Record) ([]string, error)
}

// Query asks what Host serves for Edge.
type Query struct {
	Edge string `json:"edge"`
	Host string `json:"host"`
}

// Resolution is an answer and what it was built from.
type Resolution struct {
	Instance *sourcev1.Instance `json:"instance,omitempty"` // nil: no such host
	Deps     []string           `json:"deps"`
}

// Standing is an edge's standing set and what it was built from.
type Standing struct {
	Instances []sourcev1.Instance `json:"instances,omitempty"`
	Deps      []string            `json:"deps"`
}

// EdgeList is every edge and what the list was built from.
type EdgeList struct {
	Edges []watchv1.Edge `json:"edges"`
	Deps  []string       `json:"deps"`
}

// Record is the dep on one record: a write to it re-resolves.
func Record(collection, id string) string { return collection + "/" + id }

// Collection is the dep on a whole collection: a write to any of its records re-resolves (for lists and queries).
func Collection(collection string) string { return collection + "/*" }

// Host is the dep on a hostname's claims: a write to a record that claims or claimed it re-resolves.
func Host(name string) string { return "host:" + name }

// Key is a dep of the resolver's own naming (such as "edge:lax-1" for "which record is edge lax-1"), touched when its
// Touches says so.
func Key(name string) string { return "key:" + name }

// RecordDeps are the deps a write to rec touches: its own record, its collection, and touched (from Touches).
func RecordDeps(rec *core.Record, touched ...string) []string {
	coll := rec.Collection()
	out := []string{Record(coll.Name, rec.Id), Collection(coll.Name)}
	if coll.Id != coll.Name {
		out = append(out, Record(coll.Id, rec.Id), Collection(coll.Id))
	}
	return append(out, touched...)
}
