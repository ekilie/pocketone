// Package cachev1 is the cache/v1 contract: named buckets of bytes an app keeps for a while, so extensions and apps
// can remember what was expensive to compute. Who keeps them (memory and disk in this process, Redis, Memcached) is
// the provider's business; code that caches talks only to this contract.
//
// A caller opens a bucket by name with the limits it wants (how many items, how much memory, how long, where to spill
// to disk) and then gets and sets keys in it:
//
//	b, err := cachev1.Open(app, "pages.render", cachev1.Options{MaxItems: 1000, TTL: 10 * time.Minute})
//	if v, ok, _ := b.Get(ctx, key); ok {
//		return v
//	}
//	v := render()
//	_ = b.Set(ctx, key, v, 0)
//
// The promise is a cache's, not a store's: a provider may drop any entry at any time, even right after Set, and a
// Get may miss for any reason. It never returns a value that was deleted, cleared or replaced since, nor one past its
// TTL.
package cachev1

import (
	"context"
	"fmt"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Cache gives an app its buckets. Each app has its own: a bucket named "x" on one app never sees another app's "x",
// on a multisite host too, even when the provider keeps every app's entries in one shared store.
type Cache interface {
	// Bucket returns the bucket name, creating it with opts on first use. Later calls with the same name return the
	// same bucket and ignore their opts. name must pass [ValidName]. A provider may override opts with an
	// operator's settings for that bucket, and treats each field as a hint it keeps as well as its storage allows.
	Bucket(name string, opts Options) (Bucket, error)
}

// Bucket is one named cache: keys to byte values.
type Bucket interface {
	// Get returns key's value and true, or false on a miss. The returned slice is the caller's to keep or change.
	Get(ctx context.Context, key string) ([]byte, bool, error)
	// Set stores value under key, replacing what was there. ttl 0 uses the bucket's TTL; a negative ttl deletes key.
	// The bucket keeps its own copy of value.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Delete removes key. Deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error
	// Clear removes every key of the bucket.
	Clear(ctx context.Context) error
}

// Eviction is which entry a full bucket drops to make room.
type Eviction string

const (
	// LRU drops the least recently used entry (read or written). The default.
	LRU Eviction = "lru"
	// MRU drops the most recently used entry: for scans that won't come back to what they just read.
	MRU Eviction = "mru"
	// FIFO drops the oldest written entry; reads don't count.
	FIFO Eviction = "fifo"
	// LIFO drops the newest written entry; reads don't count. (Also called FILO.)
	LIFO Eviction = "lifo"
)

// Valid reports whether e is one of the strategies above, or "" for the provider's default.
func (e Eviction) Valid() bool {
	switch e {
	case "", LRU, MRU, FIFO, LIFO:
		return true
	}
	return false
}

// Options are a bucket's limits: caps on what this app's bucket may hold, inside whatever room the provider has in
// all. A zero field leaves the choice to the provider (and to the operator's settings). Where entries live (memory,
// disk, another server) and how much room there is in all are the provider's business, not the bucket's.
type Options struct {
	// Eviction is which of the bucket's entries goes first when the bucket is full; "" is LRU.
	Eviction Eviction
	// MaxItems caps the bucket's entries; 0 is no cap of its own.
	MaxItems int
	// MaxMemory caps the bytes of the bucket's keys and values; 0 is no cap of its own.
	MaxMemory int64
	// TTL is how long an entry lives when Set gives no ttl; 0 is until it's evicted.
	TTL time.Duration
}

// Contract is cache/v1: one provider per app.
var Contract = extkit.NewContract[Cache]("cache/v1", extkit.One)

// ValidName checks a bucket name: 1 to 64 characters of a-z, 0-9, '.', '_' and '-', starting with a letter or digit.
// Prefix it with your extension's name ("pages.render") so two extensions don't share a bucket by accident.
func ValidName(name string) error {
	if name == "" || len(name) > 64 {
		return fmt.Errorf("cache/v1: bucket name %q must be 1 to 64 characters", name)
	}
	for i, r := range name {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || i > 0 && (r == '.' || r == '_' || r == '-')
		if !ok {
			return fmt.Errorf("cache/v1: bucket name %q: use a-z, 0-9, '.', '_' and '-', starting with a letter or digit", name)
		}
	}
	return nil
}

// Open returns app's bucket name from whoever provides cache/v1, or a bucket that keeps nothing when nothing does, so
// code that only works better with a cache declares cache/v1 in WorksWith and needs no other branch. Call it after
// registration (in hooks, at serve time), and keep the bucket.
func Open(app core.App, name string, opts Options) (Bucket, error) {
	if err := ValidName(name); err != nil {
		return nil, err
	}
	if !opts.Eviction.Valid() {
		return nil, fmt.Errorf("cache/v1: bucket %s: unknown eviction %q (lru, mru, fifo, lifo)", name, opts.Eviction)
	}
	c, ok := Contract.Of(app)
	if !ok {
		return None{}, nil
	}
	return c.Bucket(name, opts)
}

// None is a bucket that keeps nothing: every Get misses.
type None struct{}

var _ Bucket = None{}

// Get implements [Bucket]: always a miss.
func (None) Get(context.Context, string) ([]byte, bool, error) { return nil, false, nil }

// Set implements [Bucket]: keeps nothing.
func (None) Set(context.Context, string, []byte, time.Duration) error { return nil }

// Delete implements [Bucket].
func (None) Delete(context.Context, string) error { return nil }

// Clear implements [Bucket].
func (None) Clear(context.Context) error { return nil }
