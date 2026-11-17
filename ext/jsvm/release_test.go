package jsvm

import (
	"testing"

	"github.com/pockethost/pocketone/ext/extkit/js"
)

// holder is Go state that keeps a JS object, reached from a NativeFunc the object itself holds: the shape of a
// per-request object whose methods are closures over the request.
type holder struct{ obj js.Object }

// TestReleaseBreaksGoJSCycle: a Go value that holds a JS object, captured by a native function stored on that object,
// is a cycle across the two heaps that neither GC frees (the VM's handle table keeps the closure, the closure keeps
// the Go wrapper, the wrapper keeps the object). js.Release frees the object now, so the cycle's handles come back
// with a JS GC alone.
func TestReleaseBreaksGoJSCycle(t *testing.T) {
	vm := testVM(t)
	handles := func() int64 {
		_ = vm.Do(func() { C.JS_RunGC(vm.a) })
		return vm.Stats().Handles
	}
	round := func(release bool) {
		if err := vm.Run(func(rt js.Runtime) error {
			h := &holder{}
			h.obj = rt.NewObject()
			if err := h.obj.Set("self", js.NativeFunc(func(js.Call) (any, error) { return h.obj, nil })); err != nil {
				return err
			}
			if release {
				js.Release(h.obj)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	base := handles()
	for range 50 {
		round(true)
	}
	if got := handles(); got > base {
		t.Fatalf("released: handles %d after 50 rounds, want <= %d", got, base)
	}

	for range 50 {
		round(false)
	}
	if got := handles(); got < base+50 {
		t.Fatalf("not released: handles %d, want >= %d (the cycle this test guards against should still exist without Release)", got, base+50)
	}
}

// TestReleaseOffLoop: Release from another goroutine frees on the loop, and a released value reads as undefined.
func TestReleaseOffLoop(t *testing.T) {
	vm := testVM(t)
	var v js.Value
	if err := vm.Run(func(rt js.Runtime) error {
		var err error
		v, err = rt.Eval("({a: 1})", "t.js")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	js.Release(v)
	if !v.Nullish() {
		t.Fatal("released value should read as undefined")
	}
	js.Release(v) // twice is harmless
}
