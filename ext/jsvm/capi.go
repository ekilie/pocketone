package jsvm

// The engine's C layer, in pure Go over modernc.org/libquickjs (QuickJS translated to Go, the way PocketBase uses
// modernc.org/sqlite): no cgo, so every GOOS/GOARCH Go supports builds with CGO_ENABLED=0. The rest of the engine
// calls it through C, whose methods keep the names of the QuickJS functions and of the bridge helpers they replace.
//
// Memory rule: QuickJS code reads and writes through raw addresses, so anything it gets a pointer to lives in C memory
// (libc's heap, or the TLS's emulated stack via scratch), never on a Go stack that can move.

import (
	"bytes"
	"math"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"modernc.org/libc"
	lib "modernc.org/libquickjs"
)

// JSValue is QuickJS's value, passed by value: a 16-byte struct (union + tag) on 64-bit platforms, a NaN-boxed uint64
// on 32-bit ones. capi_value64.go and capi_value32.go build and read it (mkval, mkfloat, valTag, valPtr).
type JSValue = lib.TJSValue

// jsValueSize is sizeof(JSValue).
var jsValueSize = func() int { var v JSValue; return int(unsafe.Sizeof(v)) }()

// JSContext is the raw context plus the thread-local state every translated call takes.
type JSContext struct {
	p   uintptr
	tls *libc.TLS
	a   *AVM
}

// AVM is one app's VM: runtime, context, its registry id, and the interrupt/budget state.
type AVM struct {
	rt          uintptr
	ctx         *JSContext
	tls         *libc.TLS
	id          uintptr // the Go *VM in vmRegistry
	interrupt   int32   // set from any goroutine (atomic)
	deadline_ns int64   // monotonic (avm_now_ns); 0 = no CPU budget
	wall_ns     int64   // hard wall-clock end of the current top-level task, Go time included; 0 = none
	interrupts  int64   // how many times the interrupt handler stopped JS
	obj_class   uint32  // GoObject: exotic get/set/has/keys
	fn_class    uint32  // GoFunc: callable, constructor
	goid        uint64  // the loop goroutine
}

type cshim struct {
	JS_TAG_OBJECT, JS_TAG_UNDEFINED, JS_TAG_NULL, JS_TAG_STRING, JS_TAG_STRING_ROPE int32
	JS_TAG_INT, JS_TAG_FLOAT64, JS_TAG_BOOL                                         int32
	JS_PROMISE_FULFILLED, JS_PROMISE_REJECTED                                       int32
}

// C is the C layer.
var C = cshim{
	JS_TAG_OBJECT: lib.EJS_TAG_OBJECT, JS_TAG_UNDEFINED: lib.EJS_TAG_UNDEFINED, JS_TAG_NULL: lib.EJS_TAG_NULL,
	JS_TAG_STRING: lib.EJS_TAG_STRING, JS_TAG_STRING_ROPE: lib.EJS_TAG_STRING_ROPE, JS_TAG_INT: lib.EJS_TAG_INT,
	JS_TAG_FLOAT64: lib.EJS_TAG_FLOAT64, JS_TAG_BOOL: lib.EJS_TAG_BOOL,
	JS_PROMISE_FULFILLED: lib.EJS_PROMISE_FULFILLED, JS_PROMISE_REJECTED: lib.EJS_PROMISE_REJECTED,
}

// ---- registry: QuickJS holds ids, never Go pointers ----

var vmRegistry sync.Map // uintptr -> *VM
var vmSeq atomic.Uintptr

func registerVM(vm *VM) uintptr {
	id := vmSeq.Add(1)
	vmRegistry.Store(id, vm)
	return id
}

func unregisterVM(id uintptr) { vmRegistry.Delete(id) }

func lookupVM(id uintptr) *VM {
	v, _ := vmRegistry.Load(id)
	vm, _ := v.(*VM)
	return vm
}

// curGoroutineID is this goroutine's id (from its stack header): how a VM knows a caller is its own loop.
func curGoroutineID() uint64 {
	var buf [32]byte
	n := runtime.Stack(buf[:], false)
	b := bytes.TrimPrefix(buf[:n], []byte("goroutine "))
	if i := bytes.IndexByte(b, ' '); i > 0 {
		b = b[:i]
	}
	id, _ := strconv.ParseUint(string(b), 10, 64)
	return id
}

// ---- values ----

func (cshim) avm_tag(v JSValue) int32          { return valTag(v) }
func (cshim) avm_undefined() JSValue           { return mkval(lib.EJS_TAG_UNDEFINED, 0) }
func (cshim) avm_null() JSValue                { return mkval(lib.EJS_TAG_NULL, 0) }
func (cshim) avm_exception() JSValue           { return mkval(lib.EJS_TAG_EXCEPTION, 0) }
func (cshim) avm_float(d float64) JSValue      { return mkfloat(d) }
func (cshim) avm_is_exception(v JSValue) int32 { return b2i(valTag(v) == lib.EJS_TAG_EXCEPTION) }
func (cshim) avm_bool(b int32) JSValue {
	if b != 0 {
		return mkval(lib.EJS_TAG_BOOL, 1)
	}
	return mkval(lib.EJS_TAG_BOOL, 0)
}

// avm_int is an int when it fits in 32 bits, else a float (as JS_NewInt64).
func (c cshim) avm_int(ctx *JSContext, i int64) JSValue {
	if i >= math.MinInt32 && i <= math.MaxInt32 {
		return mkval(lib.EJS_TAG_INT, uint64(uint32(int32(i))))
	}
	return c.avm_float(float64(i))
}

func b2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func (cshim) avm_dup(ctx *JSContext, v JSValue) JSValue { return lib.XDupValue(ctx.tls, ctx.p, v) }
func (cshim) JS_FreeValue(ctx *JSContext, v JSValue)    { lib.XFreeValue(ctx.tls, ctx.p, v) }

// ---- C memory ----

// tsize is n as a C size_t, clamped to the largest one (a memory limit above 4 GiB on a 32-bit platform).
func tsize(n uint64) lib.Tsize_t {
	if uint64(lib.Tsize_t(n)) != n {
		return ^lib.Tsize_t(0)
	}
	return lib.Tsize_t(n)
}

// CString copies s into C memory; free it with C.free.
func (cshim) CString(s string) uintptr { return mustCString(s) }
func (cshim) free(p uintptr)           { libc.Xfree(nil, p) }

func mustCString(s string) uintptr {
	p, err := libc.CString(s)
	if err != nil {
		panic(err)
	}
	return p
}

// GoStringN copies n bytes of C memory into a Go string.
func (cshim) GoStringN(p uintptr, n int32) string {
	if n <= 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(ptr(p)), int(n)))
}

// scratch is n bytes of the TLS's emulated stack, for out-parameters; release with done(n), in LIFO order.
func (ctx *JSContext) scratch(n int) uintptr { return ctx.tls.Alloc(n) }
func (ctx *JSContext) done(n int)            { ctx.tls.Free(n) }

// ---- strings ----

func (cshim) JS_NewStringLen(ctx *JSContext, s string) JSValue {
	if len(s) == 0 {
		return lib.XJS_NewStringLen(ctx.tls, ctx.p, 0, 0)
	}
	p := libc.Xmalloc(ctx.tls, lib.Tsize_t(len(s)))
	if p == 0 {
		return lib.XJS_ThrowOutOfMemory(ctx.tls, ctx.p)
	}
	copy(unsafe.Slice((*byte)(ptr(p)), len(s)), s)
	v := lib.XJS_NewStringLen(ctx.tls, ctx.p, p, lib.Tsize_t(len(s)))
	libc.Xfree(ctx.tls, p)
	return v
}

// goString is ToString(v) as a Go string ("" when it throws, with the exception left pending).
func (cshim) goString(ctx *JSContext, v JSValue) string {
	pn := ctx.scratch(8)
	defer ctx.done(8)
	p := lib.XJS_ToCStringLen2(ctx.tls, ctx.p, pn, v, 0)
	if p == 0 {
		return ""
	}
	s := string(unsafe.Slice((*byte)(ptr(p)), int(*(*lib.Tsize_t)(ptr(pn)))))
	lib.XJS_FreeCString(ctx.tls, ctx.p, p)
	return s
}

// ---- objects and properties ----

func (cshim) JS_GetPropertyStr(ctx *JSContext, o JSValue, k uintptr) JSValue {
	return lib.XJS_GetPropertyStr(ctx.tls, ctx.p, o, k)
}
func (cshim) avm_get_str(ctx *JSContext, o JSValue, k uintptr) JSValue {
	return lib.XJS_GetPropertyStr(ctx.tls, ctx.p, o, k)
}
func (cshim) JS_SetPropertyStr(ctx *JSContext, o JSValue, k uintptr, v JSValue) int32 {
	return lib.XJS_SetPropertyStr(ctx.tls, ctx.p, o, k, v)
}
func (cshim) JS_GetGlobalObject(ctx *JSContext) JSValue {
	return lib.XJS_GetGlobalObject(ctx.tls, ctx.p)
}
func (cshim) JS_NewObject(ctx *JSContext) JSValue  { return lib.XJS_NewObject(ctx.tls, ctx.p) }
func (cshim) avm_new_array(ctx *JSContext) JSValue { return lib.XJS_NewArray(ctx.tls, ctx.p) }
func (cshim) avm_array_get(ctx *JSContext, arr JSValue, i uint32) JSValue {
	return lib.XJS_GetPropertyUint32(ctx.tls, ctx.p, arr, i)
}
func (cshim) avm_array_push(ctx *JSContext, arr JSValue, i uint32, v JSValue) {
	lib.XJS_SetPropertyUint32(ctx.tls, ctx.p, arr, i, v)
}

var cLength = mustCString("length")

// avm_length is ToLength(v.length), -1 when reading it throws.
func (cshim) avm_length(ctx *JSContext, v JSValue) int64 {
	lv := lib.XJS_GetPropertyStr(ctx.tls, ctx.p, v, cLength)
	if valTag(lv) == lib.EJS_TAG_EXCEPTION {
		lib.XFreeValue(ctx.tls, ctx.p, lib.XJS_GetException(ctx.tls, ctx.p))
		return -1
	}
	pf := ctx.scratch(8)
	defer ctx.done(8)
	r := lib.XJS_ToFloat64(ctx.tls, ctx.p, pf, lv)
	lib.XFreeValue(ctx.tls, ctx.p, lv)
	if r < 0 {
		lib.XFreeValue(ctx.tls, ctx.p, lib.XJS_GetException(ctx.tls, ctx.p))
		return -1
	}
	f := *(*float64)(ptr(pf))
	if f != f || f < 0 {
		return 0
	}
	return int64(f)
}

func (cshim) avm_is_array(ctx *JSContext, v JSValue) int32 {
	if valTag(v) != lib.EJS_TAG_OBJECT {
		return 0
	}
	r := lib.XJS_IsArray(ctx.tls, ctx.p, v)
	if r < 0 { // a revoked proxy
		lib.XFreeValue(ctx.tls, ctx.p, lib.XJS_GetException(ctx.tls, ctx.p))
		return 0
	}
	return b2i(r > 0)
}

func (cshim) avm_is_function(ctx *JSContext, v JSValue) int32 {
	return b2i(lib.XJS_IsFunction(ctx.tls, ctx.p, v) != 0)
}

// avm_object_keys is the own enumerable string keys as a JS array.
func (c cshim) avm_object_keys(ctx *JSContext, o JSValue) JSValue {
	out := ctx.scratch(16)
	defer ctx.done(16)
	if lib.XJS_GetOwnPropertyNames(ctx.tls, ctx.p, out, out+8, o, lib.MJS_GPN_STRING_MASK|lib.MJS_GPN_ENUM_ONLY) < 0 {
		return c.avm_exception()
	}
	tab := *(*uintptr)(ptr(out))
	n := *(*uint32)(ptr(out + 8))
	arr := lib.XJS_NewArray(ctx.tls, ctx.p)
	size := unsafe.Sizeof(lib.TJSPropertyEnum{})
	for i := uint32(0); i < n; i++ {
		e := (*lib.TJSPropertyEnum)(ptr(tab + uintptr(i)*size))
		lib.XJS_SetPropertyUint32(ctx.tls, ctx.p, arr, i, lib.XJS_AtomToString(ctx.tls, ctx.p, e.Fatom))
	}
	lib.XJS_FreePropertyEnum(ctx.tls, ctx.p, tab, n)
	return arr
}

func (cshim) avm_define_hidden(ctx *JSContext, obj JSValue, name uintptr, v JSValue) {
	lib.XJS_DefinePropertyValueStr(ctx.tls, ctx.p, obj, name, v, lib.MJS_PROP_WRITABLE|lib.MJS_PROP_CONFIGURABLE)
}

func (cshim) avm_set_proto(ctx *JSContext, obj, proto JSValue) int32 {
	return lib.XJS_SetPrototype(ctx.tls, ctx.p, obj, proto)
}

// avm_set_protos takes ownership of both prototypes.
func (cshim) avm_set_protos(a *AVM, objProto, fnProto JSValue) {
	lib.XJS_SetClassProto(a.tls, a.ctx.p, a.obj_class, objProto)
	lib.XJS_SetClassProto(a.tls, a.ctx.p, a.fn_class, fnProto)
}

// ---- conversions ----

func (cshim) JS_ToBool(ctx *JSContext, v JSValue) int32 { return lib.XJS_ToBool(ctx.tls, ctx.p, v) }

func (cshim) JS_ToInt64(ctx *JSContext, out *int64, v JSValue) int32 {
	p := ctx.scratch(8)
	defer ctx.done(8)
	r := lib.XJS_ToInt64(ctx.tls, ctx.p, p, v)
	*out = *(*int64)(ptr(p))
	return r
}

func (cshim) JS_ToFloat64(ctx *JSContext, out *float64, v JSValue) int32 {
	p := ctx.scratch(8)
	defer ctx.done(8)
	r := lib.XJS_ToFloat64(ctx.tls, ctx.p, p, v)
	*out = *(*float64)(ptr(p))
	return r
}

func (cshim) JS_JSONStringify(ctx *JSContext, v, replacer, space JSValue) JSValue {
	return lib.XJS_JSONStringify(ctx.tls, ctx.p, v, replacer, space)
}

var cGoFile = mustCString("<go>")

func (cshim) avm_parse_json(ctx *JSContext, buf uintptr, n uint64) JSValue {
	return lib.XJS_ParseJSON(ctx.tls, ctx.p, buf, lib.Tsize_t(n), cGoFile)
}

// ---- exceptions ----

func (cshim) JS_GetException(ctx *JSContext) JSValue     { return lib.XJS_GetException(ctx.tls, ctx.p) }
func (cshim) JS_HasException(ctx *JSContext) bool        { return lib.XJS_HasException(ctx.tls, ctx.p) != 0 }
func (cshim) JS_Throw(ctx *JSContext, v JSValue) JSValue { return lib.XJS_Throw(ctx.tls, ctx.p, v) }
func (cshim) JS_NewError(ctx *JSContext) JSValue         { return lib.XJS_NewError(ctx.tls, ctx.p) }

var cMessage = mustCString("message")

// avm_throw throws a plain Error with message msg[:n].
func (cshim) avm_throw(ctx *JSContext, msg uintptr, n uint64) JSValue {
	e := lib.XJS_NewError(ctx.tls, ctx.p)
	lib.XJS_SetPropertyStr(ctx.tls, ctx.p, e, cMessage, lib.XJS_NewStringLen(ctx.tls, ctx.p, msg, lib.Tsize_t(n)))
	return lib.XJS_Throw(ctx.tls, ctx.p, e)
}

// throwString throws a plain Error with message s.
func (c cshim) throwString(ctx *JSContext, s string) JSValue {
	e := lib.XJS_NewError(ctx.tls, ctx.p)
	lib.XJS_SetPropertyStr(ctx.tls, ctx.p, e, cMessage, c.JS_NewStringLen(ctx, s))
	return lib.XJS_Throw(ctx.tls, ctx.p, e)
}

var cPercentS = mustCString("%s")

// throwType throws a TypeError with message s.
func (cshim) throwType(ctx *JSContext, s string) JSValue {
	msg := mustCString(s)
	defer libc.Xfree(ctx.tls, msg)
	va := libc.NewVaList(msg)
	defer libc.Xfree(ctx.tls, va)
	return lib.XJS_ThrowTypeError(ctx.tls, ctx.p, cPercentS, va)
}

// ---- eval, calls, jobs, promises ----

func (cshim) avm_eval(ctx *JSContext, code uintptr, n uint64, file uintptr) JSValue {
	return lib.XJS_Eval(ctx.tls, ctx.p, code, lib.Tsize_t(n), file, lib.MJS_EVAL_TYPE_GLOBAL)
}

// avm_call calls fn; argv points at argc values in Go memory, copied to C memory for the call.
func (cshim) avm_call(ctx *JSContext, fn, this JSValue, argc int32, argv *JSValue) JSValue {
	if argc == 0 {
		return lib.XJS_Call(ctx.tls, ctx.p, fn, this, 0, 0)
	}
	size := int(argc) * jsValueSize
	p := ctx.scratch(size)
	copy(unsafe.Slice((*JSValue)(ptr(p)), int(argc)), unsafe.Slice(argv, int(argc)))
	r := lib.XJS_Call(ctx.tls, ctx.p, fn, this, argc, p)
	ctx.done(size)
	return r
}

// avm_run_jobs runs queued promise jobs: how many ran, -1 on a job exception.
func (cshim) avm_run_jobs(a *AVM) int32 {
	p := a.ctx.scratch(8)
	defer a.ctx.done(8)
	count := int32(0)
	for {
		r := lib.XJS_ExecutePendingJob(a.tls, a.rt, p)
		if r == 0 {
			return count
		}
		if r < 0 {
			c := *(*uintptr)(ptr(p))
			lib.XFreeValue(a.tls, c, lib.XJS_GetException(a.tls, c))
			return -1
		}
		count++
	}
}

// avm_new_promise returns a promise and writes its resolve and reject functions to funcs[0:2].
func (cshim) avm_new_promise(ctx *JSContext, funcs *JSValue) JSValue {
	size := 2 * jsValueSize
	p := ctx.scratch(size)
	defer ctx.done(size)
	r := lib.XJS_NewPromiseCapability(ctx.tls, ctx.p, p)
	copy(unsafe.Slice(funcs, 2), unsafe.Slice((*JSValue)(ptr(p)), 2))
	return r
}

func (cshim) avm_promise_state(ctx *JSContext, p JSValue) int32 {
	return lib.XJS_PromiseState(ctx.tls, ctx.p, p)
}
func (cshim) avm_promise_result(ctx *JSContext, p JSValue) JSValue {
	return lib.XJS_PromiseResult(ctx.tls, ctx.p, p)
}
func (cshim) avm_is_promise(ctx *JSContext, v JSValue) int32 {
	if valTag(v) != lib.EJS_TAG_OBJECT {
		return 0
	}
	return b2i(lib.XJS_PromiseState(ctx.tls, ctx.p, v) >= 0)
}

// ---- ES modules ----

// avm_eval_module compiles code as the ES module file, sets its import.meta and runs it: the result is a promise (top-level
// await) or an exception.
func (c cshim) avm_eval_module(ctx *JSContext, code, file string, meta map[string]string) JSValue {
	cc, cf := mustCString(code), mustCString(file)
	defer c.free(cc)
	defer c.free(cf)
	v := lib.XJS_Eval(ctx.tls, ctx.p, cc, lib.Tsize_t(len(code)), cf, lib.MJS_EVAL_TYPE_MODULE|lib.MJS_EVAL_FLAG_COMPILE_ONLY)
	if valTag(v) != lib.EJS_TAG_MODULE {
		return v
	}
	c.setImportMeta(ctx, valPtr(v), meta)
	return lib.XJS_EvalFunction(ctx.tls, ctx.p, v) // takes v
}

func (c cshim) setImportMeta(ctx *JSContext, m uintptr, meta map[string]string) {
	mv := lib.XJS_GetImportMeta(ctx.tls, ctx.p, m)
	if valTag(mv) == lib.EJS_TAG_OBJECT {
		for k, val := range meta {
			ck := mustCString(k)
			lib.XJS_SetPropertyStr(ctx.tls, ctx.p, mv, ck, c.JS_NewStringLen(ctx, val))
			c.free(ck)
		}
	}
	lib.XFreeValue(ctx.tls, ctx.p, mv)
}

// avm_compile_module compiles code as the ES module name (not run yet: QuickJS links and runs it with its importer)
// and sets its import.meta; 0 when it throws.
func (c cshim) avm_compile_module(ctx *JSContext, code, name string, meta map[string]string) uintptr {
	cc, cf := mustCString(code), mustCString(name)
	defer c.free(cc)
	defer c.free(cf)
	v := lib.XJS_Eval(ctx.tls, ctx.p, cc, lib.Tsize_t(len(code)), cf, lib.MJS_EVAL_TYPE_MODULE|lib.MJS_EVAL_FLAG_COMPILE_ONLY)
	if valTag(v) != lib.EJS_TAG_MODULE {
		if valTag(v) != lib.EJS_TAG_EXCEPTION {
			lib.XFreeValue(ctx.tls, ctx.p, v)
		}
		return 0
	}
	m := valPtr(v)
	lib.XFreeValue(ctx.tls, ctx.p, v)
	c.setImportMeta(ctx, m, meta)
	return m
}

// moduleNormalize resolves an import specifier against the importing module's name (QuickJS's JSModuleNormalizeFunc).
// It returns the module's name in js_malloc'd memory, or 0 with an exception thrown.
func moduleNormalize(tls *libc.TLS, ctx, base, name, opaque uintptr) uintptr {
	vm := lookupVM(opaque)
	resolved, err := vm.normalizeModule(libc.GoString(base), libc.GoString(name))
	if err != nil {
		C.throwString(vm.ctx, err.Error())
		return 0
	}
	cs := mustCString(resolved)
	defer libc.Xfree(tls, cs)
	return lib.Xjs_strdup(tls, ctx, cs)
}

// moduleLoad compiles the module with a normalized name (QuickJS's JSModuleLoaderFunc): its JSModuleDef, or 0 with
// an exception thrown.
func moduleLoad(tls *libc.TLS, ctx, name, opaque uintptr) uintptr {
	vm := lookupVM(opaque)
	m, err := vm.loadModule(libc.GoString(name))
	if err != nil {
		if !C.JS_HasException(vm.ctx) {
			C.throwString(vm.ctx, err.Error())
		}
		return 0
	}
	return m
}

// ---- runtime ----

func (cshim) JS_RunGC(a *AVM)                    { lib.XJS_RunGC(a.tls, a.rt) }
func (cshim) JS_SetMemoryLimit(a *AVM, n uint64) { lib.XJS_SetMemoryLimit(a.tls, a.rt, tsize(n)) }

func (cshim) avm_mem_used(a *AVM) uint64 {
	size := int(unsafe.Sizeof(lib.TJSMemoryUsage{}))
	p := a.ctx.scratch(size)
	defer a.ctx.done(size)
	lib.XJS_ComputeMemoryUsage(a.tls, a.rt, p)
	return uint64((*lib.TJSMemoryUsage)(ptr(p)).Fmalloc_size)
}

var clockStart = time.Now()

// avm_now_ns is a monotonic clock in ns.
func (cshim) avm_now_ns() int64 { return int64(time.Since(clockStart)) }

func (cshim) avm_set_interrupt(a *AVM, v int32) { atomic.StoreInt32(&a.interrupt, v) }

func (cshim) avm_on_thread(a *AVM) int32 { return b2i(curGoroutineID() == a.goid) }

// ---- the Go-object classes ----

func (cshim) avm_new_object(a *AVM, h uint32) JSValue {
	o := lib.XJS_NewObjectClass(a.tls, a.ctx.p, int32(a.obj_class))
	if valTag(o) != lib.EJS_TAG_EXCEPTION {
		lib.XJS_SetOpaque(a.tls, o, uintptr(h))
	}
	return o
}

func (cshim) avm_new_func(a *AVM, h uint32, ctor int32) JSValue {
	o := lib.XJS_NewObjectClass(a.tls, a.ctx.p, int32(a.fn_class))
	if valTag(o) != lib.EJS_TAG_EXCEPTION {
		lib.XJS_SetOpaque(a.tls, o, uintptr(h))
		if ctor != 0 {
			lib.XJS_SetConstructorBit(a.tls, a.ctx.p, o, 1)
		}
	}
	return o
}

// avm_ctor_proto gives a Go constructor its own `prototype` object (inheriting proto, the Go objects' prototype) with
// `constructor` pointing back, as a JS constructor has. Without it `x instanceof Ctor` throws ("operand 'prototype'
// property is not an object"), which the PocketBase JS SDK does with FormData on every request with a body.
func (cshim) avm_ctor_proto(ctx *JSContext, f, proto JSValue) {
	p := lib.XJS_NewObjectProto(ctx.tls, ctx.p, proto)
	if valTag(p) == lib.EJS_TAG_EXCEPTION {
		lib.XFreeValue(ctx.tls, ctx.p, lib.XJS_GetException(ctx.tls, ctx.p))
		return
	}
	lib.XJS_SetConstructor(ctx.tls, ctx.p, f, p) // dups both
	lib.XFreeValue(ctx.tls, ctx.p, p)
}

// adoptNewTargetProto makes a Go object returned by `new Ctor()` an instance of Ctor (goja sets the instance's
// prototype to new.target.prototype the same way). Only a freshly wrapped object is changed: one whose prototype is
// still the shared Go-object prototype, so slices and objects with a prototype of their own keep theirs.
var cPrototype = mustCString("prototype") // C memory for good, like the class names

func adoptNewTargetProto(tls *libc.TLS, ctx uintptr, vm *VM, ret, newTarget JSValue) {
	if valTag(ret) != lib.EJS_TAG_OBJECT || lib.XJS_GetClassID(tls, ret) != classes.obj || valTag(vm.objProto) != lib.EJS_TAG_OBJECT {
		return
	}
	cur := lib.XJS_GetPrototype(tls, ctx, ret)
	fresh := valTag(cur) == lib.EJS_TAG_OBJECT && valPtr(cur) == valPtr(vm.objProto)
	lib.XFreeValue(tls, ctx, cur)
	if !fresh || valTag(newTarget) != lib.EJS_TAG_OBJECT {
		return
	}
	np := lib.XJS_GetPropertyStr(tls, ctx, newTarget, cPrototype)
	if valTag(np) == lib.EJS_TAG_OBJECT {
		lib.XJS_SetPrototype(tls, ctx, ret, np)
	}
	if valTag(np) == lib.EJS_TAG_EXCEPTION {
		lib.XFreeValue(tls, ctx, lib.XJS_GetException(tls, ctx))
	} else {
		lib.XFreeValue(tls, ctx, np)
	}
}

// avm_handle_of is the handle of a GoObject/GoFunc, 0 for anything else.
func (cshim) avm_handle_of(a *AVM, v JSValue) uint32 {
	if valTag(v) != lib.EJS_TAG_OBJECT {
		return 0
	}
	switch id := lib.XJS_GetClassID(a.tls, v); id {
	case a.obj_class, a.fn_class:
		return uint32(lib.XJS_GetOpaque(a.tls, v, id))
	}
	return 0
}

// The two classes are registered once per process (QuickJS class ids are global) and their definitions live in C
// memory for good: each runtime keeps a pointer to the exotic methods.
var classes struct {
	once          sync.Once
	obj, fn       uint32
	objDef, fnDef uintptr
}

func initClasses() {
	tls := libc.NewTLS()
	defer tls.Close()
	ids := libc.Xcalloc(tls, 1, 8)
	lib.XJS_NewClassID(tls, ids)
	lib.XJS_NewClassID(tls, ids+4)
	classes.obj = *(*uint32)(ptr(ids))
	classes.fn = *(*uint32)(ptr(ids + 4))
	libc.Xfree(tls, ids)

	ex := libc.Xcalloc(tls, 1, lib.Tsize_t(unsafe.Sizeof(lib.TJSClassExoticMethods{})))
	*(*lib.TJSClassExoticMethods)(ptr(ex)) = lib.TJSClassExoticMethods{
		Fget_own_property:       fp(objGetOwn),
		Fget_own_property_names: fp(objKeys),
		Fdelete_property:        fp(objDelete),
		Fdefine_own_property:    fp(objDefine),
		Fhas_property:           fp(objHas),
		Fget_property:           fp(objGet),
		Fset_property:           fp(objSet),
	}
	classes.objDef = libc.Xcalloc(tls, 1, lib.Tsize_t(unsafe.Sizeof(lib.TJSClassDef{})))
	*(*lib.TJSClassDef)(ptr(classes.objDef)) = lib.TJSClassDef{
		Fclass_name: mustCString("GoObject"), Ffinalizer: fp(objFinalizer), Fexotic: ex,
	}
	classes.fnDef = libc.Xcalloc(tls, 1, lib.Tsize_t(unsafe.Sizeof(lib.TJSClassDef{})))
	*(*lib.TJSClassDef)(ptr(classes.fnDef)) = lib.TJSClassDef{
		Fclass_name: mustCString("GoFunc"), Ffinalizer: fp(fnFinalizer), Fcall: fp(fnCall),
	}
}

// defaultStackSlots is about the call depth native QuickJS reaches with a 1 MB C stack (libquickjs v0.14's default;
// v0.13 counts the same slots but defaults to 1000).
const defaultStackSlots = 5000

// avm_new creates the runtime, context, classes and interrupt handler; the calling goroutine becomes the loop.
// stackSlots is in TLS stack slots (about one per JS call frame); 0 means defaultStackSlots.
func (cshim) avm_new(id uintptr, memLimit, stackSlots uint64) *AVM {
	classes.once.Do(initClasses)
	tls := libc.NewTLS()
	a := &AVM{tls: tls, id: id, goid: curGoroutineID(), obj_class: classes.obj, fn_class: classes.fn}
	a.rt = lib.XJS_NewRuntime(tls)
	lib.XJS_SetRuntimeOpaque(tls, a.rt, id)
	if stackSlots == 0 {
		stackSlots = defaultStackSlots
	}
	lib.XJS_SetMaxStackSize(tls, a.rt, tsize(stackSlots))
	if memLimit > 0 {
		lib.XJS_SetMemoryLimit(tls, a.rt, tsize(memLimit))
	}
	lib.XJS_SetInterruptHandler(tls, a.rt, fp(interruptHandler), id)
	lib.XJS_SetModuleLoaderFunc(tls, a.rt, fp(moduleNormalize), fp(moduleLoad), id)
	lib.XJS_NewClass(tls, a.rt, a.obj_class, classes.objDef)
	lib.XJS_NewClass(tls, a.rt, a.fn_class, classes.fnDef)
	ctx := lib.XJS_NewContext(tls, a.rt)
	lib.XJS_SetContextOpaque(tls, ctx, id)
	a.ctx = &JSContext{p: ctx, tls: tls, a: a}
	return a
}

func (cshim) avm_free(a *AVM) {
	lib.XJS_FreeContext(a.tls, a.ctx.p)
	lib.XJS_FreeRuntime(a.tls, a.rt)
	a.tls.Close()
}

// fp is a Go function as a function pointer for translated C code.
func fp(f any) uintptr {
	type iface [2]uintptr
	return (*iface)(unsafe.Pointer(&f))[1]
}

// ---- callbacks from QuickJS into Go (always on the VM's loop goroutine) ----

func vmOfCtx(tls *libc.TLS, ctx uintptr) *VM { return lookupVM(lib.XJS_GetContextOpaque(tls, ctx)) }

func interruptHandler(tls *libc.TLS, rt, opaque uintptr) int32 {
	vm := lookupVM(opaque)
	if vm == nil || vm.a == nil {
		return 0
	}
	a := vm.a
	if atomic.LoadInt32(&a.interrupt) != 0 {
		a.interrupts++
		return 1
	}
	if a.deadline_ns != 0 || a.wall_ns != 0 {
		now := C.avm_now_ns()
		if (a.deadline_ns != 0 && now > a.deadline_ns) || (a.wall_ns != 0 && now > a.wall_ns) {
			a.interrupts++
			return 1
		}
	}
	return 0
}

// atomName is a property name as a Go string; ok is false for a symbol.
func atomName(tls *libc.TLS, ctx uintptr, atom uint32) (string, bool) {
	k := lib.XJS_AtomToValue(tls, ctx, atom)
	defer lib.XFreeValue(tls, ctx, k)
	if valTag(k) == lib.EJS_TAG_SYMBOL {
		return "", false
	}
	pn := tls.Alloc(8)
	defer tls.Free(8)
	p := lib.XJS_ToCStringLen2(tls, ctx, pn, k, 0)
	if p == 0 {
		lib.XFreeValue(tls, ctx, lib.XJS_GetException(tls, ctx))
		return "", false
	}
	s := string(unsafe.Slice((*byte)(ptr(p)), int(*(*lib.Tsize_t)(ptr(pn)))))
	lib.XJS_FreeCString(tls, ctx, p)
	return s, true
}

func protoGet(tls *libc.TLS, ctx uintptr, obj JSValue, atom uint32) JSValue {
	proto := lib.XJS_GetPrototype(tls, ctx, obj)
	if valTag(proto) == lib.EJS_TAG_EXCEPTION {
		return proto
	}
	if valTag(proto) != lib.EJS_TAG_OBJECT {
		lib.XFreeValue(tls, ctx, proto)
		return C.avm_undefined()
	}
	r := lib.XJS_GetPropertyInternal(tls, ctx, proto, atom, proto, 0)
	lib.XFreeValue(tls, ctx, proto)
	return r
}

func objHandle(tls *libc.TLS, obj JSValue) uint32 {
	return uint32(lib.XJS_GetOpaque(tls, obj, classes.obj))
}

func objGet(tls *libc.TLS, ctx uintptr, obj JSValue, atom uint32, receiver JSValue) JSValue {
	name, ok := atomName(tls, ctx, atom)
	if !ok {
		return protoGet(tls, ctx, obj, atom)
	}
	vm := vmOfCtx(tls, ctx)
	out := C.avm_undefined()
	switch avmGoGet(vm.ctx, objHandle(tls, obj), name, &out) {
	case -1:
		return C.avm_exception()
	case 0:
		return protoGet(tls, ctx, obj, atom)
	}
	return out
}

func objSet(tls *libc.TLS, ctx uintptr, obj JSValue, atom uint32, val, receiver JSValue, flags int32) int32 {
	vm := vmOfCtx(tls, ctx)
	name, ok := atomName(tls, ctx, atom)
	if !ok {
		C.throwType(vm.ctx, "cannot set a symbol on a Go object")
		return -1
	}
	return avmGoSet(vm.ctx, objHandle(tls, obj), name, val)
}

func objHas(tls *libc.TLS, ctx uintptr, obj JSValue, atom uint32) int32 {
	name, ok := atomName(tls, ctx, atom)
	if !ok {
		return 0
	}
	return avmGoHas(vmOfCtx(tls, ctx).ctx, objHandle(tls, obj), name)
}

func objGetOwn(tls *libc.TLS, ctx uintptr, desc uintptr, obj JSValue, atom uint32) int32 {
	name, ok := atomName(tls, ctx, atom)
	if !ok {
		return 0
	}
	vm := vmOfCtx(tls, ctx)
	h := objHandle(tls, obj)
	if desc == 0 {
		return avmGoHas(vm.ctx, h, name)
	}
	out := C.avm_undefined()
	rc := avmGoGet(vm.ctx, h, name, &out)
	if rc == 1 {
		*(*lib.TJSPropertyDescriptor)(ptr(desc)) = lib.TJSPropertyDescriptor{
			Fflags:  lib.MJS_PROP_ENUMERABLE | lib.MJS_PROP_WRITABLE | lib.MJS_PROP_CONFIGURABLE,
			Fvalue:  out,
			Fgetter: C.avm_undefined(),
			Fsetter: C.avm_undefined(),
		}
	}
	return rc
}

func objKeys(tls *libc.TLS, ctx uintptr, ptab, plen uintptr, obj JSValue) int32 {
	vm := vmOfCtx(tls, ctx)
	keys := avmGoKeys(vm.ctx, objHandle(tls, obj))
	size := unsafe.Sizeof(lib.TJSPropertyEnum{})
	alloc := len(keys)
	if alloc == 0 {
		alloc = 1
	}
	tab := lib.Xjs_malloc(tls, ctx, lib.Tsize_t(uintptr(alloc)*size))
	if tab == 0 {
		return -1
	}
	for i, k := range keys {
		p := mustCString(k)
		*(*lib.TJSPropertyEnum)(ptr(tab + uintptr(i)*size)) = lib.TJSPropertyEnum{
			Fis_enumerable: 1,
			Fatom:          lib.XJS_NewAtomLen(tls, ctx, p, lib.Tsize_t(len(k))),
		}
		libc.Xfree(tls, p)
	}
	*(*uintptr)(ptr(ptab)) = tab
	*(*uint32)(ptr(plen)) = uint32(len(keys))
	return 0
}

func objDefine(tls *libc.TLS, ctx uintptr, this JSValue, prop uint32, val, getter, setter JSValue, flags int32) int32 {
	if flags&lib.MJS_PROP_HAS_VALUE != 0 {
		return objSet(tls, ctx, this, prop, val, this, flags)
	}
	C.throwType(vmOfCtx(tls, ctx).ctx, "cannot define accessors on a Go object")
	return -1
}

func objDelete(tls *libc.TLS, ctx uintptr, obj JSValue, prop uint32) int32 { return 0 }

func objFinalizer(tls *libc.TLS, rt uintptr, val JSValue) {
	if h := uint32(lib.XJS_GetOpaque(tls, val, classes.obj)); h != 0 {
		avmGoFinalize(lib.XJS_GetRuntimeOpaque(tls, rt), h)
	}
}

func fnFinalizer(tls *libc.TLS, rt uintptr, val JSValue) {
	if h := uint32(lib.XJS_GetOpaque(tls, val, classes.fn)); h != 0 {
		avmGoFinalize(lib.XJS_GetRuntimeOpaque(tls, rt), h)
	}
}

func fnCall(tls *libc.TLS, ctx uintptr, funcObj, this JSValue, argc int32, argv uintptr, flags int32) JSValue {
	vm := vmOfCtx(tls, ctx)
	h := uint32(lib.XJS_GetOpaque(tls, funcObj, classes.fn))
	var args []JSValue
	if argc > 0 {
		args = unsafe.Slice((*JSValue)(ptr(argv)), int(argc))
	}
	ctor := flags&lib.MJS_CALL_FLAG_CONSTRUCTOR != 0
	ret := avmGoCall(vm.ctx, h, this, args, ctor)
	if ctor { // for a constructor call QuickJS passes new.target as this
		adoptNewTargetProto(tls, ctx, vm, ret, this)
	}
	return ret
}

// ptr is a C address as an unsafe.Pointer (C memory never moves).
func ptr(p uintptr) unsafe.Pointer { return unsafe.Add(nil, p) }
