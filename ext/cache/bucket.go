package cache

import (
	"context"
	"time"

	cachev1 "pocketone.dev/ext/contracts/cache/v1"
)

// bucket is one app's cachev1.Bucket: its space in the store.
type bucket struct {
	store *store
	sp    *space
}

var _ cachev1.ItemBucket = (*bucket)(nil)

// Get implements cachev1.Bucket. An item is marshaled.
func (b *bucket) Get(_ context.Context, key string) ([]byte, bool, error) {
	return b.store.getBytes(b.sp, key)
}

// GetItem implements cachev1.ItemBucket.
func (b *bucket) GetItem(_ context.Context, key string, dec cachev1.Decoder) (cachev1.Item, bool, error) {
	return b.store.getItem(b.sp, key, dec)
}

// SetItem implements cachev1.ItemBucket: the key keeps item itself in memory, marshaled only to write it to the
// disk tier.
func (b *bucket) SetItem(_ context.Context, key string, item cachev1.Item, ttl time.Duration) error {
	b.store.setItem(b.sp, key, item, ttl)
	return nil
}

// Set implements cachev1.Bucket.
func (b *bucket) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	b.store.set(b.sp, key, value, ttl)
	return nil
}

// Delete implements cachev1.Bucket.
func (b *bucket) Delete(_ context.Context, key string) error {
	b.store.del(b.sp, key)
	return nil
}

// Clear implements cachev1.Bucket.
func (b *bucket) Clear(context.Context) error {
	b.store.clear(b.sp)
	return nil
}
