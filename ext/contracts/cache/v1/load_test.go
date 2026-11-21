package cachev1

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memBucket is a minimal bucket for the tests.
type memBucket struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newMem() *memBucket { return &memBucket{m: map[string][]byte{}} }

func (b *memBucket) Get(_ context.Context, k string) ([]byte, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.m[k]
	return v, ok, nil
}
func (b *memBucket) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.m[k] = append([]byte(nil), v...)
	return nil
}
func (b *memBucket) Delete(_ context.Context, k string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.m, k)
	return nil
}
func (b *memBucket) Clear(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.m = map[string][]byte{}
	return nil
}

func TestLoadCoalesces(t *testing.T) {
	b := newMem()
	var calls atomic.Int32
	release := make(chan struct{})
	fn := func(context.Context) ([]byte, error) {
		calls.Add(1)
		<-release
		return []byte("v"), nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := Load(context.Background(), b, "k", fn)
			if err != nil || string(v) != "v" {
				t.Errorf("got %q %v", v, err)
			}
			// released only after the store: a waiter never misses again
			if _, ok, _ := b.Get(context.Background(), "k"); !ok {
				t.Error("released before the value was stored")
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Fatalf("fn ran %d times, want 1", n)
	}
	if _, _ = Load(context.Background(), b, "k", fn); calls.Load() != 1 {
		t.Fatal("a hit computed again")
	}
}

func TestLoadErrorNotCached(t *testing.T) {
	b := newMem()
	boom := errors.New("boom")
	var calls atomic.Int32
	fail := func(context.Context) ([]byte, error) { calls.Add(1); return nil, boom }
	if _, err := Load(context.Background(), b, "k", fail); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, ok, _ := b.Get(context.Background(), "k"); ok {
		t.Fatal("an error was stored")
	}
	v, err := Load(context.Background(), b, "k", func(context.Context) ([]byte, error) { return []byte("ok"), nil })
	if err != nil || string(v) != "ok" {
		t.Fatalf("retry: %q %v", v, err)
	}
}

func TestLoadOwnContext(t *testing.T) {
	b := newMem()
	release := make(chan struct{})
	fn := func(ctx context.Context) ([]byte, error) {
		<-release
		if ctx.Err() != nil {
			return nil, ctx.Err() // the first caller leaving must not cancel the computation
		}
		return []byte("v"), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := Load(ctx, b, "k", fn); first <- err }()
	time.Sleep(20 * time.Millisecond)
	second := make(chan []byte, 1)
	go func() { v, _ := Load(context.Background(), b, "k", fn); second <- v }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("first caller: %v", err)
	}
	close(release)
	if v := <-second; string(v) != "v" {
		t.Fatalf("second caller got %q", v)
	}
}

type loadNum int

func (loadNum) CacheSize() int64 { return 8 }

func TestItemsLoad(t *testing.T) {
	it := NewItems[loadNum](newMem(), "", func([]byte) (loadNum, error) { return 0, errors.New("not marshalable") })
	var calls atomic.Int32
	fn := func(context.Context) (loadNum, error) { calls.Add(1); return 7, nil }
	for i := 0; i < 3; i++ {
		v, err := it.Load(context.Background(), "k", fn)
		if err != nil || v != 7 {
			t.Fatalf("got %v %v", v, err)
		}
	}
	// a bucket of bytes keeps nothing for a T that isn't a Marshaler: every Load computes
	if calls.Load() != 3 {
		t.Fatalf("calls = %d", calls.Load())
	}
}
