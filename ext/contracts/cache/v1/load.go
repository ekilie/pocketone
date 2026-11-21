package cachev1

import (
	"context"
	"reflect"
	"sync"
)

// Load returns key's value from b, or on a miss computes it with fn and stores it. Callers asking for the same key of
// the same bucket while fn runs share that one call (a cold key under load computes once, not once per request):
//
//   - every caller is released only after the value is stored, so none of them misses again right after;
//   - fn's error goes to the callers waiting then and is never stored: the next caller tries again;
//   - each caller waits on its own ctx: one that gives up returns ctx's error and leaves fn running for the others.
//
// fn runs with ctx's values but not its cancellation (the first caller leaving mustn't fail the rest), so it bounds
// its own time. A failed store still returns the value. A bucket whose type can't be a map key isn't coalesced.
func Load(ctx context.Context, b Bucket, key string, fn func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	get := func(ctx context.Context) (any, bool, error) { return b.Get(ctx, key) }
	v, err := load(ctx, b, key, get,
		func(ctx context.Context) (any, error) { return fn(ctx) },
		func(ctx context.Context, v any) error { return b.Set(ctx, key, v.([]byte), 0) })
	data, _ := v.([]byte)
	return data, err
}

// Load is [Load] for a bucket of T values: a miss computes the value with fn once for every caller waiting on the
// same key, stores it, then releases them all.
func (it *Items[T]) Load(ctx context.Context, key string, fn func(ctx context.Context) (T, error)) (T, error) {
	get := func(ctx context.Context) (any, bool, error) { return it.Get(ctx, key) }
	v, err := load(ctx, it, key, get,
		func(ctx context.Context) (any, error) { return fn(ctx) },
		func(ctx context.Context, v any) error { return it.Set(ctx, key, v.(T), 0) })
	t, _ := v.(T)
	return t, err
}

type flightKey struct {
	owner any // the Bucket, or the *Items
	key   string
}

type flight struct {
	done chan struct{}
	val  any
	err  error
}

var (
	flightsMu sync.Mutex
	flights   = map[flightKey]*flight{}
)

func load(ctx context.Context, owner any, key string,
	get func(context.Context) (any, bool, error),
	compute func(context.Context) (any, error),
	store func(context.Context, any) error,
) (any, error) {
	if v, ok, err := get(ctx); err == nil && ok {
		return v, nil
	}
	if t := reflect.TypeOf(owner); t == nil || !t.Comparable() {
		v, err := compute(ctx)
		if err == nil {
			_ = store(ctx, v)
		}
		return v, err
	}
	k := flightKey{owner, key}
	flightsMu.Lock()
	f, joined := flights[k]
	if !joined {
		f = &flight{done: make(chan struct{})}
		flights[k] = f
	}
	flightsMu.Unlock()
	if !joined {
		go func() {
			wctx := context.WithoutCancel(ctx)
			defer func() {
				flightsMu.Lock()
				delete(flights, k)
				flightsMu.Unlock()
				close(f.done)
			}()
			// stored by a flight that ended between this caller's miss and now: no need to compute it again
			if v, ok, err := get(wctx); err == nil && ok {
				f.val = v
				return
			}
			f.val, f.err = compute(wctx)
			if f.err == nil {
				_ = store(wctx, f.val)
			}
		}()
	}
	select {
	case <-f.done:
		return f.val, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
