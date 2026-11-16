package jsvm

/*
#include "bridge.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unsafe"
)

const preludeJS = `
(() => {
	const hide = (name, value) => Object.defineProperty(globalThis, name, { value, writable: true, configurable: true })
	hide("__settle", (p, id) => p.then(v => __done(id, true, v), e => __done(id, false, e)))
	const obj = Object.create(Object.prototype)
	Object.defineProperty(obj, "toJSON", { value: function () { return __goJSON(this) } })
	Object.defineProperty(obj, "toString", { value: function () { return __goString(this) } })
	// a pointer to a primitive (*string, *int64, ...) compares and prints as its value, as in goja
	Object.defineProperty(obj, Symbol.toPrimitive, { value: function (hint) { return __goPrim(this, hint) } })
	Object.defineProperty(obj, Symbol.iterator, { value: function* () { for (let i = 0; i < this.length; i++) yield this[i] } })
	// Go slices get the generic Array methods (map, forEach, filter, ...), as in goja
	const arr = Object.create(Array.prototype)
	Object.defineProperty(arr, "toJSON", { value: function () { return __goJSON(this) } })
	Object.defineProperty(arr, "toString", { value: function () { return Array.prototype.join.call(this, ",") } })
	const fn = Object.create(Function.prototype)
	// Any Go function or method can run off the loop: await $app.findRecordById.async("posts", id)
	Object.defineProperty(fn, "async", { value: function (...args) { return __goAsync(this, args) } })
	hide("console", { log: __log, error: __log, warn: __log, info: __log, debug: __log, trace: __log })
	hide("__protos", [obj, fn, arr])
	// goja has no performance global; keep QuickJS's but out of the global's keys
	if ("performance" in globalThis) Object.defineProperty(globalThis, "performance", { enumerable: false })
	// Go slices count as arrays, as in goja
	const isArray = Array.isArray
	Object.defineProperty(Array, "isArray", { value: (v) => isArray(v) || __isGoSlice(v), writable: true, configurable: true })
})();
`

func (vm *VM) installPrelude() error {
	hidden := func(name string, fn nativeFn) {
		h, _ := vm.newHandle(entry{kind: kindNative, native: fn})
		vm.setHidden(name, C.avm_new_func(vm.a, C.uint32_t(h), 0))
	}
	hidden("__done", nativeDone)
	hidden("__goAsync", nativeAsync)
	hidden("__goJSON", nativeJSON)
	hidden("__goString", nativeString)
	hidden("__goPrim", func(vm *VM, this C.JSValue, args []C.JSValue, ctor bool) C.JSValue {
		if h := C.avm_handle_of(vm.a, args[0]); h != 0 {
			d := deref(vm.handles[h].v)
			switch d.Kind() {
			case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
				jv, err := vm.toJS(d)
				if err != nil {
					return vm.throw(err)
				}
				return jv
			}
		}
		return nativeString(vm, this, args, ctor)
	})
	hidden("__isGoSlice", func(vm *VM, this C.JSValue, args []C.JSValue, ctor bool) C.JSValue {
		if len(args) > 0 {
			if h := C.avm_handle_of(vm.a, args[0]); h != 0 && vm.handles[h].kind == kindObj {
				if d := deref(vm.handles[h].v); d.Kind() == reflect.Slice || d.Kind() == reflect.Array {
					return C.avm_bool(1)
				}
			}
		}
		return C.avm_bool(0)
	})
	hidden("__log", func(vm *VM, this C.JSValue, args []C.JSValue, ctor bool) C.JSValue {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = vm.goString(a)
		}
		vm.Log(strings.Join(parts, " "))
		return C.avm_undefined()
	})
	if err := vm.evalHere(preludeJS, "<prelude>"); err != nil {
		return err
	}
	g := C.JS_GetGlobalObject(vm.ctx)
	cs := C.CString("__protos")
	arr := C.JS_GetPropertyStr(vm.ctx, g, cs)
	C.free(unsafe.Pointer(cs))
	vm.free_(g)
	objP := C.avm_array_get(vm.ctx, arr, 0)
	fnP := C.avm_array_get(vm.ctx, arr, 1)
	vm.sliceProto = C.avm_array_get(vm.ctx, arr, 2)
	vm.free_(arr)
	C.avm_set_protos(vm.a, objP, fnP) // takes ownership
	return nil
}

func numArg(vm *VM, v C.JSValue) uint64 {
	var f C.double
	C.JS_ToFloat64(vm.ctx, &f, v)
	return uint64(f)
}

func nativeDone(vm *VM, this C.JSValue, args []C.JSValue, ctor bool) C.JSValue {
	id := numArg(vm, args[0])
	ch, ok := vm.settles[id]
	if !ok {
		return C.avm_undefined()
	}
	resT := vm.settleTypes[id]
	delete(vm.settles, id)
	delete(vm.settleTypes, id)
	var s settled
	if C.JS_ToBool(vm.ctx, args[1]) != 0 {
		s = vm.settleValue(C.avm_dup(vm.ctx, args[2]), resT)
	} else {
		C.JS_Throw(vm.ctx, C.avm_dup(vm.ctx, args[2]))
		s = settled{err: vm.exception()}
	}
	ch <- s
	return C.avm_undefined()
}

// nativeAsync runs a Go function on its own goroutine and returns a promise. Arguments are converted on the loop;
// the result is converted back on the loop when it is done.
func nativeAsync(vm *VM, this C.JSValue, args []C.JSValue, ctor bool) C.JSValue {
	h := C.avm_handle_of(vm.a, args[0])
	if h == 0 || vm.handles[h].kind != kindFn {
		return C.avm_throw_type(vm.ctx, C.CString("async: not a Go function"))
	}
	if vm.asyncInFlight >= vm.cfg.MaxAsync {
		return vm.throw(fmt.Errorf("too many async Go calls in flight (%d)", vm.cfg.MaxAsync))
	}
	fn := vm.handles[h].v
	n := int(C.avm_length(vm.ctx, args[1]))
	jsArgs := make([]C.JSValue, n)
	for i := range jsArgs {
		jsArgs[i] = C.avm_array_get(vm.ctx, args[1], C.uint32_t(i))
	}
	in, err := vm.goArgs(fn.Type(), jsArgs)
	for _, a := range jsArgs {
		vm.free_(a)
	}
	if err != nil {
		return vm.throw(err)
	}
	funcs := make([]C.JSValue, 2)
	p := C.avm_new_promise(vm.ctx, &funcs[0])
	resolve, reject := funcs[0], funcs[1]
	vm.asyncInFlight++
	go func() {
		var out []reflect.Value
		var perr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					perr = fmt.Errorf("%v", r)
				}
			}()
			out = fn.Call(in)
		}()
		vm.post(func() {
			vm.asyncInFlight--
			defer vm.free_(resolve)
			defer vm.free_(reject)
			var v C.JSValue
			if perr != nil {
				v = vm.throw(perr)
			} else {
				v = vm.goResults(out)
			}
			target := resolve
			if C.avm_is_exception(v) != 0 {
				v = C.JS_GetException(vm.ctx)
				target = reject
			}
			r := C.avm_call(vm.ctx, target, C.avm_undefined(), 1, &v)
			vm.free_(v)
			vm.free_(r)
		})
	}()
	return p
}

func nativeJSON(vm *VM, this C.JSValue, args []C.JSValue, ctor bool) C.JSValue {
	h := C.avm_handle_of(vm.a, args[0])
	if h == 0 {
		return C.avm_undefined()
	}
	b, err := json.Marshal(vm.handles[h].v.Interface())
	if err != nil {
		return vm.throw(err)
	}
	cs := C.CString(string(b))
	defer C.free(unsafe.Pointer(cs))
	return C.avm_parse_json(vm.ctx, cs, C.size_t(len(b)))
}

func nativeString(vm *VM, this C.JSValue, args []C.JSValue, ctor bool) C.JSValue {
	h := C.avm_handle_of(vm.a, args[0])
	if h == 0 {
		return vm.str("[object]")
	}
	v := vm.handles[h].v.Interface()
	if s, ok := v.(fmt.Stringer); ok {
		return vm.str(s.String())
	}
	if e, ok := v.(error); ok {
		return vm.str(e.Error())
	}
	return vm.str(fmt.Sprintf("[Go %T]", v))
}
