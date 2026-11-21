// Package fsv1 is the instance.fs/v1 contract: one instance's code files on local disk, for extensions that read and
// write them (ext/deploy today). It answers which folder, where an extension may keep files it can rebuild about
// the instance (its cache), one writers' lock, and whether the instance runs (and how to restart it around a change).
//
// There is no request lookup and no auth in it: callers say which instance by id. On a single app the app provides it
// and is its only instance ([Self]); a multisite host provides it on its own scope (ProvideIn) for each of its
// instances, and its handlers pass the id the request routed to. Who may call is the caller's business
// (PocketBase's auth on a single app, the host's auth function on a host).
//
// The package also holds the code-path rules: what belongs to an instance's code ([IsCodePath]), what a file API may
// reach ([IsReachable]) and how a client path maps to a file without escaping the folder ([Resolve]). pb_data and
// pb_cache are never reachable through this contract.
package fsv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"
	"errors"
)

// Self is the instance id of a single app: the app itself.
const Self = ""

// Files gives each instance's code files.
//
//contract:id instance.fs/v1
//contract:kind service
type Files interface {
	// Instance returns the instance's files ([Self] on a single app). ErrNoInstance when the provider has no such
	// instance.
	Instance(ctx context.Context, id string) (FS, error)
}

// ErrNoInstance: the provider has no instance with that id.
var ErrNoInstance = errors.New("instance.fs/v1: no such instance")

// FS is one instance's code files.
type FS interface {
	// Dir is the instance folder on local disk (it holds pb_hooks, pb_public, ... and pb_data, which this contract
	// never reaches). An error when there's no folder yet (a single app before it bootstraps).
	Dir(ctx context.Context) (string, error)

	// CacheDir is a folder for files an extension can rebuild about the instance (name: the extension's, e.g.
	// "deploy"), made if missing, following PocketOne's pb_cache convention: pb_cache/<name> in Dir on a single app; a
	// host keeps it outside Dir (Pocker and Docker: the host's pb_cache/instances/<id>/<name>). Anything in it may be
	// deleted at any time (a cache clear, an admin, a cleanup) and the extension carries on. It is never code and never
	// reachable ([IsCodePath], [IsReachable]), and it is on the same filesystem as Dir, so renames between them are
	// atomic.
	CacheDir(ctx context.Context, name string) (string, error)

	// Lock takes the instance's writers' lock: every write to the code files (a deploy's commit, a file API write)
	// holds it, so they never interleave. Call unlock when done.
	Lock(ctx context.Context) (func(), error)

	// Running reports whether the instance is running now.
	Running(ctx context.Context) (bool, error)

	// Restart applies a change that mustn't happen under a running app: it stops the instance if it runs, calls apply
	// (if not nil) while it's stopped, and has it run again, whether apply failed or not. An instance that isn't running
	// gets apply only and stays as it was. An instance that can't stop itself (a single app) calls apply as it is and
	// then restarts. err is apply's; startErr says the instance didn't come back.
	Restart(ctx context.Context, apply func(context.Context) error) (startErr, err error)
}
