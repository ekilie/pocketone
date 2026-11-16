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

type ptrFields struct {
	Rule  *string
	Count *int
}

// A primitive set on a pointer field allocates it (collection rules are *string); null clears it.
func TestPrimitiveIntoPointer(t *testing.T) {
	vm := testVM(t)
	v := &ptrFields{}
	vm.Set("o", v)
	if _, err := vm.RunString(`o.rule = "id = @request.auth.id"; o.count = 3`); err != nil {
		t.Fatal(err)
	}
	if v.Rule == nil || *v.Rule != "id = @request.auth.id" || v.Count == nil || *v.Count != 3 {
		t.Fatalf("%+v", v)
	}
	if _, err := vm.RunString(`o.rule = null`); err != nil || v.Rule != nil {
		t.Fatalf("null didn't clear: %v %v", v.Rule, err)
	}
}

type innerCfg struct {
	Enabled  bool
	Duration int
	Template struct{ Subject, Body string }
}

type outerCfg struct {
	OTP  innerCfg
	Auth *innerCfg
}

// A plain object assigned to a struct field merges into it (goja's behavior PocketBase migrations rely on).
func TestStructFieldMerge(t *testing.T) {
	vm := testVM(t)
	v := &outerCfg{Auth: &innerCfg{Duration: 9}}
	v.OTP.Duration = 180
	v.OTP.Template.Subject = "subj"
	vm.Set("c", v)
	if _, err := vm.RunString(`c.otp = { enabled: true, template: { body: "b" } }; c.auth = { enabled: true }`); err != nil {
		t.Fatal(err)
	}
	if !v.OTP.Enabled || v.OTP.Duration != 180 || v.OTP.Template.Subject != "subj" || v.OTP.Template.Body != "b" {
		t.Fatalf("OTP: %+v", v.OTP)
	}
	if !v.Auth.Enabled || v.Auth.Duration != 9 {
		t.Fatalf("Auth: %+v", v.Auth)
	}
}
