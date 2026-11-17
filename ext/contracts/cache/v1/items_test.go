package cachev1

import (
	"context"
	"testing"
	"time"
)

type num struct{ n byte }

func (v num) MarshalCache() ([]byte, error) { return []byte{v.n}, nil }
func (v num) CacheSize() int64              { return 1 }

func decodeNum(b []byte) (num, error) { return num{b[0]}, nil }

// bytesOnly is a bucket that keeps only bytes, as a remote provider would.
type bytesOnly map[string][]byte

func (m bytesOnly) Get(_ context.Context, k string) ([]byte, bool, error) {
	v, ok := m[k]
	return v, ok, nil
}

func (m bytesOnly) Set(_ context.Context, k string, v []byte, ttl time.Duration) error {
	if ttl < 0 {
		delete(m, k)
		return nil
	}
	m[k] = v
	return nil
}

func (m bytesOnly) Delete(_ context.Context, k string) error { delete(m, k); return nil }
func (m bytesOnly) Clear(context.Context) error              { clear(m); return nil }

func TestItemsOverBytes(t *testing.T) {
	ctx := context.Background()
	b := bytesOnly{}
	it := NewItems(b, decodeNum)
	if err := it.Set(ctx, "k", num{7}, 0); err != nil {
		t.Fatal(err)
	}
	if string(b["k"]) != "\x07" {
		t.Fatalf("stored %q, want the marshaled bytes", b["k"])
	}
	if v, ok, err := it.Get(ctx, "k"); err != nil || !ok || v.n != 7 {
		t.Fatalf("Get: %v %v %v", v, ok, err)
	}
	if err := it.Set(ctx, "k", num{1}, -1); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := it.Get(ctx, "k"); ok {
		t.Fatal("a negative ttl didn't delete")
	}
	if _, ok, _ := NewItems(None{}, decodeNum).Get(ctx, "k"); ok {
		t.Fatal("None kept an item")
	}
}
