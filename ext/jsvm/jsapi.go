package jsvm

/*
#include "bridge.h"
*/
import "C"

import (
	"reflect"
	"regexp"
	"runtime"
	"unsafe"

	"github.com/pockethost/pocketone/ext/extkit/js"
)

// The extkit/js types over this engine: what extensions see.

type jsRuntime struct{ vm *VM }

var _ js.Runtime = jsRuntime{}

func (r jsRuntime) Engine() any { return r.vm }

func (r jsRuntime) Set(name string, v any) error {
	jv, err := r.vm.jsFromAny(v)
	if err != nil {
		return err
	}
	r.vm.setGlobal(name, jv)
	return nil
}

// Get reads a global: a property of globalThis, or a top-level let/const/class binding of the hook files.
func (r jsRuntime) Get(name string) js.Value {
	g := C.JS_GetGlobalObject(r.vm.ctx)
	cs := C.CString(name)
	v := C.JS_GetPropertyStr(r.vm.ctx, g, cs)
	C.free(unsafe.Pointer(cs))
	r.vm.free_(g)
	if C.avm_tag(v) == C.JS_TAG_UNDEFINED && identRe.MatchString(name) {
		code := "(() => { try { return " + name + " } catch (e) { return undefined } })()"
		cc := C.CString(code)
		cf := C.CString("<get>")
		lv := C.avm_eval(r.vm.ctx, cc, C.size_t(len(code)), cf)
		C.free(unsafe.Pointer(cc))
		C.free(unsafe.Pointer(cf))
		if C.avm_is_exception(lv) != 0 {
			r.vm.free_(C.JS_GetException(r.vm.ctx))
		} else {
			v = lv
		}
	}
	return r.vm.value(v)
}

var identRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func (r jsRuntime) NewObject() js.Object {
	return r.vm.value(C.JS_NewObject(r.vm.ctx)).(*jsValue)
}

// jsFromAny converts a Go value for extkit/js: a js.NativeFunc becomes a JS function, a *jsValue its own value.
func (vm *VM) jsFromAny(v any) (C.JSValue, error) {
	switch x := v.(type) {
	case js.NativeFunc:
		return vm.nativeValue(func(c *call) (any, error) {
			res, err := x(jsCall{c})
			if jv, ok := res.(*jsValue); ok {
				return C.avm_dup(vm.ctx, jv.v), err
			}
			return res, err
		}, false), nil
	case *jsValue:
		return C.avm_dup(vm.ctx, x.v), nil
	}
	return vm.toJS(reflect.ValueOf(v))
}

// jsValue is a JS value held for Go. It's freed (on the loop) when Go drops it.
type jsValue struct {
	vm *VM
	v  C.JSValue
}

// value takes ownership of v.
func (vm *VM) value(v C.JSValue) js.Value {
	jv := &jsValue{vm: vm, v: v}
	runtime.SetFinalizer(jv, func(jv *jsValue) {
		vm.post(func() {
			if vm.a != nil {
				vm.free_(jv.v)
			}
		})
	})
	return jv
}

func (v *jsValue) Nullish() bool {
	t := C.avm_tag(v.v)
	return t == C.JS_TAG_UNDEFINED || t == C.JS_TAG_NULL
}

func (v *jsValue) Export() any {
	gv, err := v.vm.fromJS(v.v, nil)
	if err != nil || !gv.IsValid() {
		return nil
	}
	return gv.Interface()
}

func (v *jsValue) String() string { return v.vm.goString(v.v) }

func (v *jsValue) Object() js.Object {
	if C.avm_tag(v.v) != C.JS_TAG_OBJECT {
		return nil
	}
	return v
}

func (v *jsValue) Func() js.Func {
	if C.avm_tag(v.v) != C.JS_TAG_OBJECT || C.avm_is_function(v.vm.ctx, v.v) == 0 {
		return nil
	}
	return jsFunc{vm: v.vm, ref: v.vm.ref(v.v)}
}

func (v *jsValue) Get(name string) js.Value {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	return v.vm.value(C.avm_get_str(v.vm.ctx, v.v, cs))
}

func (v *jsValue) Set(name string, x any) error {
	jv, err := v.vm.jsFromAny(x)
	if err != nil {
		return err
	}
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	C.JS_SetPropertyStr(v.vm.ctx, v.v, cs, jv)
	return nil
}

func (v *jsValue) Keys() []string {
	keys := C.avm_object_keys(v.vm.ctx, v.v)
	if C.avm_is_exception(keys) != 0 {
		v.vm.free_(C.JS_GetException(v.vm.ctx))
		return nil
	}
	defer v.vm.free_(keys)
	n := int(C.avm_length(v.vm.ctx, keys))
	out := make([]string, n)
	for i := range out {
		kv := C.avm_array_get(v.vm.ctx, keys, C.uint32_t(i))
		out[i] = v.vm.goString(kv)
		v.vm.free_(kv)
	}
	return out
}

type jsFunc struct {
	vm  *VM
	ref *jsRef
}

// Call runs the function on its VM (from any goroutine), awaiting a returned promise; the result is exported.
func (f jsFunc) Call(args ...any) (any, error) {
	in := make([]reflect.Value, len(args))
	for i, a := range args {
		in[i] = reflect.ValueOf(a)
	}
	res, err := f.vm.CallRef(f.ref, in, reflect.TypeOf(func() any { return nil }))
	if err != nil {
		return nil, err
	}
	if !res.IsValid() {
		return nil, nil
	}
	return res.Interface(), nil
}

type jsCall struct{ c *call }

func (c jsCall) Len() int { return c.c.len() }

func (c jsCall) Arg(i int) js.Value {
	if i >= c.c.len() {
		return c.c.vm.value(C.avm_undefined())
	}
	return c.c.vm.value(C.avm_dup(c.c.vm.ctx, c.c.args[i]))
}

func (c jsCall) Runtime() js.Runtime { return jsRuntime{c.c.vm} }

// Runtime is this VM as an extkit/js runtime (valid on the loop).
func (vm *VM) Runtime() js.Runtime { return jsRuntime{vm} }

// Run runs fn on the loop with the VM as an extkit/js runtime.
func (vm *VM) Run(fn func(rt js.Runtime) error) error {
	var err error
	if derr := vm.Do(func() { err = fn(jsRuntime{vm}) }); derr != nil {
		return derr
	}
	return err
}
