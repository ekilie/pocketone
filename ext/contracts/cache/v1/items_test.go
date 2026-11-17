package cachev1

import (
	"context"
	"testing"
	"time"
)

type num struct{ n byte }

func (v num) MarshalCache() ([]byte, error) { return []byte{v.n}, nil }
func (v num) CacheSize() int64              { return 1 }
func (v num) CacheVersion() string          { return "n1" }

func decodeNum(b []byte) (num, error) { return num{b[0]}, nil }

type live struct{}

func (live) CacheSize() int64 { return 1 }

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
	it := NewItems(b, "n1", decodeNum)
	if err := it.Set(ctx, "k", num{7}, 0); err != nil {
		t.Fatal(err)
	}
	if string(b["k"]) != "n1\x00\x07" {
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
	if _, ok, _ := NewItems(None{}, "n1", decodeNum).Get(ctx, "k"); ok {
		t.Fatal("None kept an item")
	}
}

func TestItemsNotMarshalerOverBytes(t *testing.T) {
	ctx := context.Background()
	b := bytesOnly{"k": []byte("old")}
	it := NewItems(b, "", func([]byte) (live, error) { return live{}, nil })
	if err := it.Set(ctx, "k", live{}, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := b["k"]; ok {
		t.Fatal("a bytes-only bucket kept the old value under a key Set to an item it can't keep")
	}
}

func TestItemsVersionOverBytes(t *testing.T) {
	ctx := context.Background()
	b := bytesOnly{}
	_ = NewItems(b, "n1", decodeNum).Set(ctx, "k", num{7}, 0)
	if _, ok, _ := NewItems(b, "n2", decodeNum).Get(ctx, "k"); ok {
		t.Fatal("a reader of another version decoded the bytes")
	}
}
