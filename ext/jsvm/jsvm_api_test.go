//go:build cgo

package jsvm

import (
	"runtime"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/extkit/js"
)

// A js.Value passes to Func.Call as itself, and a dropped Func frees its reference.
func TestFuncValueArgsAndRefs(t *testing.T) {
	vm := testVM(t)
	var f js.Func
	var obj js.Value
	if err := vm.Run(func(rt js.Runtime) error {
		if _, err := rt.Eval(`globalThis.same = (a, b) => a === b; globalThis.o = { x: 1 }`, "t.js"); err != nil {
			return err
		}
		f = rt.Get("same").Func()
		obj = rt.Get("o")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := f.Call(obj, obj)
	if err != nil || got != true {
		t.Fatalf("a js.Value didn't pass as itself: %v %v", got, err)
	}

	count := func() int {
		n := 0
		_ = vm.Do(func() { n = len(vm.refs) })
		return n
	}
	before := count()
	for i := 0; i < 200; i++ {
		_ = vm.Run(func(rt js.Runtime) error { _ = rt.Get("same").Func(); return nil })
	}
	for i := 0; i < 20 && count() > before+10; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	if n := count(); n > before+10 {
		t.Fatalf("Func refs leak: %d before, %d after", before, n)
	}
}
