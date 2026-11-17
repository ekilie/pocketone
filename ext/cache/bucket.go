package cache

import (
	"context"
	"time"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
)

// bucket is one app's cachev1.Bucket: its space in the store.
type bucket struct {
	store *store
	sp    *space
}

var _ cachev1.Bucket = (*bucket)(nil)

// Get implements cachev1.Bucket.
func (b *bucket) Get(_ context.Context, key string) ([]byte, bool, error) {
	v, ok := b.store.get(b.sp, key)
	return v, ok, nil
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
