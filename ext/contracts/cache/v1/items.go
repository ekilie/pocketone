package cachev1

import (
	"context"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Item is a Go value a bucket can keep as it is: a parsed template, a compiled program, a decoded struct. A bucket
// that keeps items ([ItemBucket]) holds the value itself in memory and calls MarshalCache only when it has to turn it
// into bytes (to move it to disk, to send it to another server, or for a plain [Bucket.Get]). A bucket that keeps
// only bytes marshals it on every Set; [Items] hides the difference.
//
// Once Set, an item is shared: every Get returns the same value, from any goroutine, and MarshalCache may run on
// another goroutine while it's read. Treat it as immutable from then on.
type Item interface {
	// MarshalCache returns the item's bytes, which the decoder given to [ItemBucket.GetItem] (or [OpenItems]) turns
	// back into an equal item.
	MarshalCache() ([]byte, error)
	// CacheSize is roughly how many bytes the item holds in memory, counted against the bucket's and the provider's
	// memory limits in place of a byte value's length.
	CacheSize() int64
}

// Decoder turns bytes from [Item.MarshalCache] back into an item.
type Decoder func([]byte) (Item, error)

// ItemBucket is the optional interface of a [Bucket] that keeps [Item] values without marshaling them. Byte values
// and items share the bucket's keys: Get on a key holding an item returns its MarshalCache bytes, and GetItem on a key
// holding bytes returns them decoded.
type ItemBucket interface {
	Bucket
	// GetItem returns key's value and true, or false on a miss. decode turns bytes into an item when the value isn't
	// held as one (it was Set as bytes, or the provider marshaled it to move it). The item is shared, not a copy.
	GetItem(ctx context.Context, key string, decode Decoder) (Item, bool, error)
	// SetItem stores item under key, replacing what was there, without marshaling it unless the provider has to. ttl
	// is as in [Bucket.Set].
	SetItem(ctx context.Context, key string, item Item, ttl time.Duration) error
}

// Items is a bucket of T values: typed Get and Set over any [Bucket]. On an [ItemBucket] the values stay Go values;
// on a bucket of bytes each Set marshals and each Get decodes.
type Items[T Item] struct {
	b      Bucket
	decode func([]byte) (T, error)
}

// OpenItems opens bucket name as [Open] does, for T values that decode turns back from their MarshalCache bytes.
func OpenItems[T Item](app core.App, name string, opts Options, decode func([]byte) (T, error)) (*Items[T], error) {
	b, err := Open(app, name, opts)
	if err != nil {
		return nil, err
	}
	return NewItems(b, decode), nil
}

// NewItems returns typed access to b's values.
func NewItems[T Item](b Bucket, decode func([]byte) (T, error)) *Items[T] {
	return &Items[T]{b: b, decode: decode}
}

// Bucket returns the bucket underneath.
func (it *Items[T]) Bucket() Bucket { return it.b }

// Get returns key's value and true, or false on a miss.
func (it *Items[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var zero T
	ib, ok := it.b.(ItemBucket)
	if !ok {
		data, ok, err := it.b.Get(ctx, key)
		if err != nil || !ok {
			return zero, false, err
		}
		v, err := it.decode(data)
		if err != nil {
			return zero, false, err
		}
		return v, true, nil
	}
	v, ok, err := ib.GetItem(ctx, key, func(b []byte) (Item, error) { return it.decode(b) })
	if err != nil || !ok {
		return zero, false, err
	}
	if t, isT := v.(T); isT {
		return t, true, nil
	}
	data, err := v.MarshalCache() // another type under this key: read it as T through its bytes
	if err != nil {
		return zero, false, err
	}
	t, err := it.decode(data)
	if err != nil {
		return zero, false, err
	}
	return t, true, nil
}

// Set stores v under key; ttl is as in [Bucket.Set].
func (it *Items[T]) Set(ctx context.Context, key string, v T, ttl time.Duration) error {
	if ib, ok := it.b.(ItemBucket); ok {
		return ib.SetItem(ctx, key, v, ttl)
	}
	if ttl < 0 {
		return it.b.Delete(ctx, key)
	}
	data, err := v.MarshalCache()
	if err != nil {
		return err
	}
	return it.b.Set(ctx, key, data, ttl)
}

// Delete removes key.
func (it *Items[T]) Delete(ctx context.Context, key string) error { return it.b.Delete(ctx, key) }
