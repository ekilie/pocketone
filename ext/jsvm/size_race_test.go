package jsvm

import (
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// guardedStore is the shape that crashed the mothership on 2026-11-21 ("concurrent map iteration and map write"):
// a map owned by another goroutine, written under its own lock, reachable through pointers from a Go call's result.
type guardedStore struct {
	mu   sync.RWMutex
	data map[string]any
}

type sizeHolder struct {
	name  string
	store *guardedStore
}

// deepSize runs on the VM goroutine without the owner's lock, so it must never read a lock-guarded map.
// Before the fix this test dies with "fatal error: concurrent map iteration and map write" (and -race reports it).
func TestDeepSizeSharedGuardedMap(t *testing.T) {
	s := &guardedStore{data: map[string]any{}}
	for i := 0; i < 64; i++ {
		s.data[strconv.Itoa(i)] = &sizeHolder{name: strings.Repeat("x", 32)}
	}
	root := reflect.ValueOf(&sizeHolder{name: "root", store: s})

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			s.mu.Lock()
			s.data[strconv.Itoa(i%256)] = &sizeHolder{name: "w"}
			delete(s.data, strconv.Itoa((i+128)%256))
			s.mu.Unlock()
		}
	}()

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if deepSize(root, 1<<40, nil) <= 0 {
			t.Fatal("deepSize returned nothing")
		}
	}
	close(stop)
	wg.Wait()
}

// A fresh map returned by a call is still charged for what it holds.
func TestDeepSizeOwnedMapCounted(t *testing.T) {
	m := map[string]string{"k": strings.Repeat("y", 4096)}
	if n := deepSize(reflect.ValueOf(m), 1<<40, nil); n < 4096 {
		t.Fatalf("owned map undercounted: %d", n)
	}
}

// A type with a registered sizer is charged by its sizer, which reads under the type's own locks.
func TestDeepSizeUsesSizer(t *testing.T) {
	type sized struct{ data map[string]string }
	RegisterSizer(reflect.TypeOf(&sized{}), func(reflect.Value) int64 { return 12345 })
	defer sizers.Delete(reflect.TypeOf(&sized{}))
	v := []*sized{{data: map[string]string{"a": "b"}}}
	if n := deepSize(reflect.ValueOf(v), 1<<40, nil); n < 12345 {
		t.Fatalf("sizer not used: %d", n)
	}
}

// A sized type behind an unexported field is walked, not passed to its sizer (reflect would panic on Interface).
func TestDeepSizeSizerUnexportedField(t *testing.T) {
	type sized struct{ s string }
	type wrap struct{ inner *sized }
	RegisterSizer(reflect.TypeOf(&sized{}), func(v reflect.Value) int64 { _ = v.Interface(); return 1 })
	defer sizers.Delete(reflect.TypeOf(&sized{}))
	if n := deepSize(reflect.ValueOf(&wrap{inner: &sized{s: "abc"}}), 1<<40, nil); n <= 0 {
		t.Fatalf("got %d", n)
	}
}
