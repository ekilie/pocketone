// Package watchv1 is the instance.watch/v1 contract: an edge follows a control plane (a mothership) lazily. It opens
// one stream, asks for a hostname the first time a request names it, and from then on hears about every change to
// what that hostname serves, until it says it no longer cares. Nothing is sent that no edge asked for, except the
// list of edges and the instances an edge must hold whatever its traffic (its standing set).
//
// A wire contract at /api/_c/instance.watch/v1. The mothership provides it (ext/mothership); edges call it
// (ext/mothership/client).
//
// The order of things is the promise:
//
//   - The stream's first event is a hello: the connection id, this edge's record, every edge and the standing set.
//   - Watch registers the hostname on the connection before it reads anything, then sends the answer as an event on
//     the stream (never in the reply), so the answer and every later change arrive in the order they happened.
//   - A Watch naming no live connection (the edge's stream is down) gets the answer in its reply, with nothing
//     registered: the caller may use it once, but nobody will tell it when it changes.
//   - A change is sent only when the answer differs from the one sent last for that hostname on that connection.
//   - A dropped stream forgets every watch. On reconnect the edge starts over: what it cached is no longer watched.
package watchv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"time"

	sourcev1 "pocketone.dev/ext/contracts/instance/source/v1"
	"pocketone.dev/ext/extkit"
)

// Watcher lets edges follow the instances behind the hostnames they serve.
//
//contract:id instance.watch/v1
//contract:kind wire
//contract:scope fleet
type Watcher interface {
	// Stream opens this edge's stream: a hello first, then answers and changes for the hostnames it watches, the
	// edges list when it changes, and its standing set when that changes.
	//
	//op: GET stream
	Stream(ctx context.Context, a extkit.Auth, in StreamInput) (extkit.Stream, error)

	// Watch registers a hostname on a stream and sends its answer there (or, with no live stream, in the reply).
	//
	//op: POST watch
	Watch(ctx context.Context, a extkit.Auth, in WatchInput) (WatchResult, error)

	// Unwatch stops changes for a hostname on a stream.
	//
	//op: POST unwatch
	Unwatch(ctx context.Context, a extkit.Auth, in UnwatchInput) (UnwatchResult, error)
}

// StreamInput names the edge opening the stream (its edge id, such as lax-1). Edges sign in as superusers today, so
// the edge says who it is; a per-edge credential will carry it instead.
type StreamInput struct {
	Edge string `json:"edge"`
}

// WatchInput is a hostname to watch on a connection (Conn from the stream's hello). An empty or unknown Conn gets
// the answer in the reply, unwatched.
type WatchInput struct {
	Conn string `json:"conn"`
	Edge string `json:"edge"` // the edge asking, for an answer in the reply (no live Conn)
	Host string `json:"host"`
}

// WatchResult says whether the hostname is watched. Registered: its answer comes on the stream. Not registered:
// Answer is the answer, for this once.
type WatchResult struct {
	Registered bool    `json:"registered"`
	Answer     *Answer `json:"answer,omitempty"`
}

// UnwatchInput is a hostname to stop watching on a connection.
type UnwatchInput struct {
	Conn string `json:"conn"`
	Host string `json:"host"`
}

// UnwatchResult says whether the hostname was watched on the connection.
type UnwatchResult struct {
	Removed bool `json:"removed"`
}

// Event kinds, in Event.Kind.
const (
	KindHello    = "hello"    // the first event: Conn, Self, Edges, Standing
	KindHost     = "host"     // Host's answer (the first after a Watch, or a change)
	KindEdges    = "edges"    // the edges list changed: Edges
	KindStanding = "standing" // this edge's standing set changed: Standing (all of it)
	KindPing     = "ping"     // nothing happened: sent when the stream was idle for PingEvery, so a dead one shows
)

// PingEvery is how often an idle stream gets a ping; an edge that hears nothing for a few of them reconnects.
const PingEvery = 25 * time.Second

// Event is one entry on the stream (the SSE event is "stream", the data this JSON).
type Event struct {
	Kind string `json:"kind"`
	// Seq numbers a connection's events from 1, so an edge can tell it missed nothing.
	Seq uint64 `json:"seq"`

	Conn     string              `json:"conn,omitempty"`     // hello
	Self     *Edge               `json:"self,omitempty"`     // hello: this edge's record, if it has one
	Edges    []Edge              `json:"edges,omitempty"`    // hello, edges
	Standing []sourcev1.Instance `json:"standing,omitempty"` // hello, standing
	Host     string              `json:"host,omitempty"`     // host
	Answer   *Answer             `json:"answer,omitempty"`   // host
}

// Answer is what a hostname serves: an instance, or nothing (Found false; "no such host" is an answer too, watched
// like any other, so the instance that later takes the name reaches the edge).
type Answer struct {
	Found    bool               `json:"found"`
	Instance *sourcev1.Instance `json:"instance,omitempty"`
}

// Edge is one edge of the fleet, as the stream sends it. A Remote of "edge:<ID>" or "edge:@<Name>" names it.
type Edge struct {
	ID          string    `json:"id"`   // the control plane's record id
	Name        string    `json:"name"` // the edge id, such as lax-1
	InternalURL string    `json:"internal_url,omitempty"`
	Region      string    `json:"region,omitempty"`
	Status      string    `json:"status,omitempty"`
	LastSeen    time.Time `json:"last_seen,omitzero"`
}

// ErrUnknownConn: the connection named isn't open (on Unwatch; Watch answers in its reply instead).
var ErrUnknownConn = &extkit.Error{Code: "unknown_conn", Message: "no open stream has that connection id", Status: 404}
