package jsvm

import (
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// testVM is a fresh VM for one test, closed when it ends; generous limits so tests measure behavior, not caps.
func testVM(t testing.TB) *VM {
	t.Helper()
	vm, err := New(VMConfig{MemoryLimit: 512 << 20, TaskBudget: time.Minute, TaskWall: 2 * time.Minute, PumpSyncCalls: true, Blocking: PBBlocking, Shared: PBShared}, nil)
	if err != nil {
		t.Fatal(err)
	}
	vm.Log = func(a ...any) { t.Log(a...) }
	t.Cleanup(vm.Close)
	return vm
}

// BindApp gives a bare VM the binds Register gives an app's VM (for the engine tests).
func BindApp(vm *VM, app core.App) error {
	return vm.Do(func() {
		BindCore(vm)
		BindDbx(vm)
		BindSecurity(vm)
		BindOS(vm)
		BindFilepath(vm)
		BindHTTP(vm)
		BindFilesystem(vm)
		BindForms(vm)
		BindMails(vm)
		BindApis(vm)
		hooksBinds(app, vm)
		cronBinds(app, vm)
		routerBinds(app, vm)
		vm.setGo("$app", app)
	})
}
