//go:build cgo

package jsvm

import (
	"os"
	"runtime"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// Memory and start time per idle app VM (bound to the app, with the benchmark hooks loaded).
func TestFootprint(t *testing.T) {
	if os.Getenv("ASYNCVM_BENCH") == "" {
		t.Skip("set ASYNCVM_BENCH=1")
	}
	app, _ := tests.NewTestApp()
	defer app.Cleanup()
	const n = 200
	runtime.GC()
	base := rssBytes()
	start := time.Now()
	vms := make([]*VM, n)
	var js int64
	for i := range vms {
		vm, err := New(VMConfig{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		vm.Log = func(...any) {}
		if err := BindApp(vm, app); err != nil {
			t.Fatal(err)
		}
		if err := vm.Eval(benchHooks, "main.pb.js"); err != nil {
			t.Fatal(err)
		}
		vms[i] = vm
	}
	el := time.Since(start)
	runtime.GC()
	for _, vm := range vms {
		js += vm.Stats().JSBytes
	}
	t.Logf("%d VMs: start %v each, RSS +%d KB each, JS heap %d KB each, threads ~%d", n, (el / n).Round(time.Microsecond), (rssBytes()-base)/n>>10, js/n>>10, runtime.NumGoroutine())
	for _, vm := range vms {
		vm.Close()
	}
}
