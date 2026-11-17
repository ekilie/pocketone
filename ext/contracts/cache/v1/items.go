package cachev1

import (
	"context"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Item is a Go value a bucket keeps as it is: a parsed template, a compiled program, a decoded struct. Each key holds
// its own: a bucket never hands one key's item to another key, another bucket or another app, so the code that owns
// the bucket owns its items, and every Get of a key returns that key's item (not a copy).
//
// An item that is only an Item lives in memory alone: it never goes to disk or to another server, and when memory is
// full it is simply dropped. To let it leave memory, make it a [Marshaler].
type Item interface {
	// CacheSize is roughly how many bytes the item holds in memory, counted against the bucket's memory limits in
	// place of a byte value's length.
	CacheSize() int64
}

// Marshaler is an [Item] that can leave memory. MarshalCache promises that its bytes, given to a [Decoder] of the
// same CacheVersion, come back as an item equivalent in every way. Only then may a provider move it to disk, keep it across
// restarts or send it to another server; a provider may also keep identical bytes from many keys once, there.
//
// A provider marshals when it moves the item, which may be later than Set and on another goroutine. Changing an item
// after Set changes what its key holds in memory, and may or may not reach a copy already marshaled: Set it again to
// be sure.
type Marshaler interface {
	Item
	MarshalCache() ([]byte, error)
	// CacheVersion names MarshalCache's format. Change it whenever the format changes: a provider keeps it beside the
	// bytes (bucket, key, version -> content), and a Decoder of another version never sees them.
	CacheVersion() string
}

// Decoder turns bytes back into an item: a [Marshaler]'s bytes of the same Version, or bytes Set as a value (whose
// version is "").
type Decoder struct {
	Version string
	Decode  func([]byte) (Item, error)
}

// ItemBucket is the optional interface of a [Bucket] that keeps [Item] values without marshaling them. Byte values
// and items share the bucket's keys: Get on a key holding a Marshaler returns its bytes (and misses on an item that
// can't marshal), and GetItem on a key holding bytes of the decoder's version returns them decoded, as that key's
// item from then on. Bytes of another version are a miss. An item held in memory is returned as it is, whatever the
// decoder's version.
type ItemBucket interface {
	Bucket
	// GetItem returns key's item and true, or false on a miss. decode turns bytes into an item when the key doesn't
	// hold one in memory (it was Set as bytes, or its item was moved to disk or kept across a restart).
	GetItem(ctx context.Context, key string, decode Decoder) (Item, bool, error)
	// SetItem stores item under key, replacing what was there; the bucket keeps item itself, not a copy. ttl is as in
	// [Bucket.Set].
	SetItem(ctx context.Context, key string, item Item, ttl time.Duration) error
}

// Items is a bucket of T values: typed Get and Set over any [Bucket]. On an [ItemBucket] the values stay Go values;
// on a bucket of bytes each Set marshals (keeping nothing for a T that isn't a [Marshaler]) and each Get decodes,
// with the version written in front of the bytes.
type Items[T Item] struct {
	b       Bucket
	version string
	decode  func([]byte) (T, error)
}

// OpenItems opens bucket name as [Open] does, for T values that decode turns back from bytes of version (T's
// CacheVersion; "" for a T that isn't a [Marshaler]).
func OpenItems[T Item](app core.App, name string, opts Options, version string, decode func([]byte) (T, error)) (*Items[T], error) {
	b, err := Open(app, name, opts)
	if err != nil {
		return nil, err
	}
	return NewItems(b, version, decode), nil
}

// NewItems returns typed access to b's values.
func NewItems[T Item](b Bucket, version string, decode func([]byte) (T, error)) *Items[T] {
	return &Items[T]{b: b, version: version, decode: decode}
}

// Decoder is the items' [Decoder].
func (it *Items[T]) Decoder() Decoder {
	return Decoder{Version: it.version, Decode: func(b []byte) (Item, error) { return it.decode(b) }}
}

// versioned is what Items writes to a bucket of bytes: the version, a zero byte, the bytes. Version "" writes the
// bytes alone.
func versioned(version string, data []byte) []byte {
	if version == "" {
		return data
	}
	out := make([]byte, 0, len(version)+1+len(data))
	return append(append(append(out, version...), 0), data...)
}

// unversioned returns versioned's bytes, if they're of version.
func unversioned(version string, data []byte) ([]byte, bool) {
	if version == "" {
		return data, true
	}
	if len(data) <= len(version) || string(data[:len(version)]) != version || data[len(version)] != 0 {
		return nil, false
	}
	return data[len(version)+1:], true
}

// Bucket returns the bucket underneath.
func (it *Items[T]) Bucket() Bucket { return it.b }

// Get returns key's value and true, or false on a miss. A key holding an item of another type is a miss.
func (it *Items[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var zero T
	ib, ok := it.b.(ItemBucket)
	if !ok {
		data, ok, err := it.b.Get(ctx, key)
		if err != nil || !ok {
			return zero, false, err
		}
		if data, ok = unversioned(it.version, data); !ok {
			return zero, false, nil
		}
		return it.decoded(data)
	}
	v, ok, err := ib.GetItem(ctx, key, it.Decoder())
	if err != nil || !ok {
		return zero, false, err
	}
	t, isT := v.(T)
	if !isT { // another type under this key: a miss, never a conversion
		return zero, false, nil
	}
	return t, true, nil
}

func (it *Items[T]) decoded(data []byte) (T, bool, error) {
	t, err := it.decode(data)
	if err != nil {
		var zero T
		return zero, false, err
	}
	return t, true, nil
}

// Set stores v under key; ttl is as in [Bucket.Set].
func (it *Items[T]) Set(ctx context.Context, key string, v T, ttl time.Duration) error {
	if ib, ok := it.b.(ItemBucket); ok {
		return ib.SetItem(ctx, key, v, ttl)
	}
	m, canMarshal := any(v).(Marshaler)
	if ttl < 0 || !canMarshal { // a bucket of bytes can't keep it: the old value mustn't stay either
		return it.b.Delete(ctx, key)
	}
	data, err := m.MarshalCache()
	if err != nil {
		return err
	}
	return it.b.Set(ctx, key, versioned(m.CacheVersion(), data), ttl)
}

// Delete removes key.
func (it *Items[T]) Delete(ctx context.Context, key string) error { return it.b.Delete(ctx, key) }

// Clear removes every key of the bucket, items and byte values alike.
func (it *Items[T]) Clear(ctx context.Context) error { return it.b.Clear(ctx) }
