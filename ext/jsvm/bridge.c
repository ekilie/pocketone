#include <pthread.h>
#include <string.h>
#include <time.h>
#include "bridge.h"
#include "_cgo_export.h"

int64_t avm_now_ns(void) {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (int64_t)ts.tv_sec * 1000000000LL + ts.tv_nsec;
}

static AVM *avm_of(JSContext *ctx) { return (AVM *)JS_GetContextOpaque(ctx); }

static int avm_interrupt_handler(JSContext *ctx, void *opaque) {
	AVM *a = (AVM *)opaque;
	if (__atomic_load_n(&a->interrupt, __ATOMIC_RELAXED)) {
		a->interrupts++;
		return 1;
	}
	if (a->deadline_ns || a->wall_ns) {
		int64_t now = avm_now_ns();
		if ((a->deadline_ns && now > a->deadline_ns) || (a->wall_ns && now > a->wall_ns)) {
			a->interrupts++;
			return 1;
		}
	}
	return 0;
}

// ---- GoObject: every property access goes to Go by name ----

// The property name as a C string, or NULL for a symbol.
static const char *atom_name(JSContext *ctx, JSAtom atom, size_t *n) {
	JSValue k = JS_AtomToValue(ctx, atom);
	if (JS_IsSymbol(k)) {
		JS_FreeValue(ctx, k);
		return NULL;
	}
	const char *s = JS_ToCStringLen(ctx, n, k);
	JS_FreeValue(ctx, k);
	return s;
}

static JSValue proto_get(JSContext *ctx, JSValueConst obj, JSAtom atom) {
	JSValue proto = JS_GetPrototype(ctx, obj);
	if (JS_IsException(proto)) return proto;
	JSValue r = JS_IsObject(proto) ? JS_GetProperty(ctx, proto, atom) : JS_UNDEFINED;
	JS_FreeValue(ctx, proto);
	return r;
}

static JSValue obj_get(JSContext *ctx, JSValueConst obj, JSAtom atom, JSValueConst receiver) {
	AVM *a = avm_of(ctx);
	uint32_t h = (uint32_t)(uintptr_t)JS_GetOpaque(obj, a->obj_class);
	size_t n;
	const char *name = atom_name(ctx, atom, &n);
	if (!name) return proto_get(ctx, obj, atom);
	JSValue out = JS_UNDEFINED;
	int rc = avmGoGet(ctx, h, (char *)name, n, &out);
	JS_FreeCString(ctx, name);
	if (rc < 0) return JS_EXCEPTION;
	if (rc == 0) return proto_get(ctx, obj, atom);
	return out;
}

static int obj_set(JSContext *ctx, JSValueConst obj, JSAtom atom, JSValueConst val, JSValueConst receiver, int flags) {
	AVM *a = avm_of(ctx);
	uint32_t h = (uint32_t)(uintptr_t)JS_GetOpaque(obj, a->obj_class);
	size_t n;
	const char *name = atom_name(ctx, atom, &n);
	if (!name) {
		JS_ThrowTypeError(ctx, "cannot set a symbol on a Go object");
		return -1;
	}
	int rc = avmGoSet(ctx, h, (char *)name, n, val);
	JS_FreeCString(ctx, name);
	return rc;
}

static int obj_has(JSContext *ctx, JSValueConst obj, JSAtom atom) {
	AVM *a = avm_of(ctx);
	uint32_t h = (uint32_t)(uintptr_t)JS_GetOpaque(obj, a->obj_class);
	size_t n;
	const char *name = atom_name(ctx, atom, &n);
	if (!name) return 0;
	int rc = avmGoHas(ctx, h, (char *)name, n);
	JS_FreeCString(ctx, name);
	return rc;
}

static int obj_get_own(JSContext *ctx, JSPropertyDescriptor *desc, JSValueConst obj, JSAtom atom) {
	AVM *a = avm_of(ctx);
	uint32_t h = (uint32_t)(uintptr_t)JS_GetOpaque(obj, a->obj_class);
	size_t n;
	const char *name = atom_name(ctx, atom, &n);
	if (!name) return 0;
	int rc;
	if (desc) {
		JSValue out = JS_UNDEFINED;
		rc = avmGoGet(ctx, h, (char *)name, n, &out);
		if (rc == 1) {
			desc->flags = JS_PROP_ENUMERABLE | JS_PROP_WRITABLE | JS_PROP_CONFIGURABLE;
			desc->value = out;
			desc->getter = JS_UNDEFINED;
			desc->setter = JS_UNDEFINED;
		}
	} else {
		rc = avmGoHas(ctx, h, (char *)name, n);
	}
	JS_FreeCString(ctx, name);
	return rc;
}

static int obj_keys(JSContext *ctx, JSPropertyEnum **ptab, uint32_t *plen, JSValueConst obj) {
	AVM *a = avm_of(ctx);
	uint32_t h = (uint32_t)(uintptr_t)JS_GetOpaque(obj, a->obj_class);
	JSValue arr = avmGoKeys(ctx, h);
	if (JS_IsException(arr)) return -1;
	int64_t len = 0;
	JS_GetLength(ctx, arr, &len);
	JSPropertyEnum *tab = js_malloc(ctx, sizeof(JSPropertyEnum) * (len > 0 ? len : 1));
	if (!tab) {
		JS_FreeValue(ctx, arr);
		return -1;
	}
	for (int64_t i = 0; i < len; i++) {
		JSValue k = JS_GetPropertyUint32(ctx, arr, (uint32_t)i);
		tab[i].atom = JS_ValueToAtom(ctx, k);
		tab[i].is_enumerable = true;
		JS_FreeValue(ctx, k);
	}
	JS_FreeValue(ctx, arr);
	*ptab = tab;
	*plen = (uint32_t)len;
	return 0;
}

static int obj_define(JSContext *ctx, JSValueConst this_obj, JSAtom prop, JSValueConst val, JSValueConst getter, JSValueConst setter, int flags) {
	if (flags & JS_PROP_HAS_VALUE) return obj_set(ctx, this_obj, prop, val, this_obj, flags);
	JS_ThrowTypeError(ctx, "cannot define accessors on a Go object");
	return -1;
}

static int obj_delete(JSContext *ctx, JSValueConst obj, JSAtom prop) {
	return 0;
}

static void obj_finalizer(JSRuntime *rt, JSValueConst val) {
	AVM *a = (AVM *)JS_GetRuntimeOpaque(rt);
	uint32_t h = (uint32_t)(uintptr_t)JS_GetOpaque(val, a->obj_class);
	if (h) avmGoFinalize(a->id, h);
}

static JSClassExoticMethods obj_exotic = {
	.get_own_property = obj_get_own,
	.get_own_property_names = obj_keys,
	.delete_property = obj_delete,
	.define_own_property = obj_define,
	.has_property = obj_has,
	.get_property = obj_get,
	.set_property = obj_set,
};

// ---- GoFunc: a callable object whose call goes to Go ----

static JSValue fn_call(JSContext *ctx, JSValueConst func_obj, JSValueConst this_val, int argc, JSValueConst *argv, int flags) {
	AVM *a = avm_of(ctx);
	uint32_t h = (uint32_t)(uintptr_t)JS_GetOpaque(func_obj, a->fn_class);
	return avmGoCall(ctx, h, (JSValue)this_val, argc, (JSValue *)argv, (flags & JS_CALL_FLAG_CONSTRUCTOR) ? 1 : 0);
}

static void fn_finalizer(JSRuntime *rt, JSValueConst val) {
	AVM *a = (AVM *)JS_GetRuntimeOpaque(rt);
	uint32_t h = (uint32_t)(uintptr_t)JS_GetOpaque(val, a->fn_class);
	if (h) avmGoFinalize(a->id, h);
}

AVM *avm_new(uintptr_t id, size_t mem_limit, size_t stack_size) {
	AVM *a = calloc(1, sizeof(AVM));
	a->id = id;
	a->thread = (uint64_t)pthread_self();
	a->rt = JS_NewRuntime();
	JS_SetRuntimeOpaque(a->rt, a);
	// The stack top is where this (locked) thread is now; checks stay valid because the thread never changes.
	JS_UpdateStackTop(a->rt);
	if (stack_size) JS_SetMaxStackSize(a->rt, stack_size);
	if (mem_limit) JS_SetMemoryLimit(a->rt, mem_limit);
	JS_SetInterruptHandler(a->rt, avm_interrupt_handler, a);

	JS_NewClassID(a->rt, &a->obj_class);
	JSClassDef od = {.class_name = "GoObject", .finalizer = obj_finalizer, .exotic = &obj_exotic};
	JS_NewClass(a->rt, a->obj_class, &od);
	JS_NewClassID(a->rt, &a->fn_class);
	JSClassDef fd = {.class_name = "GoFunc", .finalizer = fn_finalizer, .call = fn_call};
	JS_NewClass(a->rt, a->fn_class, &fd);

	a->ctx = JS_NewContext(a->rt);
	JS_SetContextOpaque(a->ctx, a);
	return a;
}

void avm_free(AVM *a) {
	JS_FreeContext(a->ctx);
	JS_FreeRuntime(a->rt);
	free(a);
}

int avm_on_thread(AVM *a) { return (uint64_t)pthread_self() == a->thread; }
void avm_set_interrupt(AVM *a, int v) { __atomic_store_n(&a->interrupt, v, __ATOMIC_RELAXED); }

JSValue avm_new_object(AVM *a, uint32_t handle) {
	JSValue o = JS_NewObjectClass(a->ctx, a->obj_class);
	if (!JS_IsException(o)) JS_SetOpaque(o, (void *)(uintptr_t)handle);
	return o;
}

JSValue avm_new_func(AVM *a, uint32_t handle, int ctor) {
	JSValue o = JS_NewObjectClass(a->ctx, a->fn_class);
	if (!JS_IsException(o)) {
		JS_SetOpaque(o, (void *)(uintptr_t)handle);
		if (ctor) JS_SetConstructorBit(a->ctx, o, true);
	}
	return o;
}

uint32_t avm_handle_of(AVM *a, JSValue v) {
	JSClassID id = JS_GetClassID(v);
	if (id == a->obj_class) return (uint32_t)(uintptr_t)JS_GetOpaque(v, a->obj_class);
	if (id == a->fn_class) return (uint32_t)(uintptr_t)JS_GetOpaque(v, a->fn_class);
	return 0;
}

int avm_tag(JSValue v) { return JS_VALUE_GET_NORM_TAG(v); }
JSValue avm_undefined(void) { return JS_UNDEFINED; }
JSValue avm_null(void) { return JS_NULL; }
JSValue avm_bool(int b) { return JS_NewBool(NULL, b); }
JSValue avm_float(double d) { return JS_NewFloat64(NULL, d); }
JSValue avm_int(JSContext *ctx, int64_t i) { return JS_NewInt64(ctx, i); }
JSValue avm_dup(JSContext *ctx, JSValue v) { return JS_DupValue(ctx, v); }
int avm_is_exception(JSValue v) { return JS_IsException(v); }
int avm_is_function(JSContext *ctx, JSValue v) { return JS_IsFunction(ctx, v); }

JSValue avm_throw(JSContext *ctx, const char *msg, size_t n) {
	JSValue err = JS_NewError(ctx);
	JS_SetPropertyStr(ctx, err, "message", JS_NewStringLen(ctx, msg, n));
	return JS_Throw(ctx, err);
}
JSValue avm_throw_type(JSContext *ctx, const char *msg) { return JS_ThrowTypeError(ctx, "%s", msg); }

JSValue avm_eval(JSContext *ctx, const char *code, size_t n, const char *file) {
	return JS_Eval(ctx, code, n, file, JS_EVAL_TYPE_GLOBAL);
}

int avm_run_jobs(AVM *a) {
	int count = 0;
	for (;;) {
		JSContext *c;
		int r = JS_ExecutePendingJob(a->rt, &c);
		if (r == 0) return count;
		if (r < 0) {
			JSValue e = JS_GetException(c);
			JS_FreeValue(c, e);
			return -1;
		}
		count++;
	}
}

JSValue avm_array_get(JSContext *ctx, JSValue arr, uint32_t i) { return JS_GetPropertyUint32(ctx, arr, i); }
int64_t avm_length(JSContext *ctx, JSValue v) {
	int64_t n = 0;
	if (JS_GetLength(ctx, v, &n) < 0) return -1;
	return n;
}
int avm_is_array(JSValue v) { return JS_IsArray(v); }
JSValue avm_new_array(JSContext *ctx) { return JS_NewArray(ctx); }
void avm_array_push(JSContext *ctx, JSValue arr, uint32_t i, JSValue v) { JS_SetPropertyUint32(ctx, arr, i, v); }

JSValue avm_object_keys(JSContext *ctx, JSValue o) {
	JSPropertyEnum *tab;
	uint32_t len;
	if (JS_GetOwnPropertyNames(ctx, &tab, &len, o, JS_GPN_STRING_MASK | JS_GPN_ENUM_ONLY) < 0) return JS_EXCEPTION;
	JSValue arr = JS_NewArray(ctx);
	for (uint32_t i = 0; i < len; i++) JS_SetPropertyUint32(ctx, arr, i, JS_AtomToString(ctx, tab[i].atom));
	JS_FreePropertyEnum(ctx, tab, len);
	return arr;
}

JSValue avm_get_str(JSContext *ctx, JSValue o, const char *k) { return JS_GetPropertyStr(ctx, o, k); }
void avm_set_str(JSContext *ctx, JSValue o, const char *k, JSValue v) { JS_SetPropertyStr(ctx, o, k, v); }
JSValue avm_call(JSContext *ctx, JSValue fn, JSValue this_val, int argc, JSValue *argv) {
	return JS_Call(ctx, fn, this_val, argc, argv);
}
JSValue avm_new_promise(JSContext *ctx, JSValue *funcs) { return JS_NewPromiseCapability(ctx, funcs); }
int avm_promise_state(JSContext *ctx, JSValue p) { return JS_PromiseState(ctx, p); }
JSValue avm_promise_result(JSContext *ctx, JSValue p) { return JS_PromiseResult(ctx, p); }
int avm_is_promise(JSValue v) { return JS_IsPromise(v); }

size_t avm_mem_used(AVM *a) {
	JSMemoryUsage u;
	JS_ComputeMemoryUsage(a->rt, &u);
	return (size_t)u.malloc_size;
}

void avm_set_protos(AVM *a, JSValue obj_proto, JSValue fn_proto) {
	JS_SetClassProto(a->ctx, a->obj_class, obj_proto);
	JS_SetClassProto(a->ctx, a->fn_class, fn_proto);
}

JSValue avm_parse_json(JSContext *ctx, const char *buf, size_t n) {
	return JS_ParseJSON(ctx, buf, n, "<go>");
}

void avm_define_hidden(JSContext *ctx, JSValue obj, const char *name, JSValue v) {
	JS_DefinePropertyValueStr(ctx, obj, name, v, JS_PROP_WRITABLE | JS_PROP_CONFIGURABLE);
}

JSValue avm_eval_flags(JSContext *ctx, const char *code, size_t n, const char *file, int flags) {
	return JS_Eval(ctx, code, n, file, flags);
}

int avm_set_proto(JSContext *ctx, JSValue obj, JSValue proto) {
	return JS_SetPrototype(ctx, obj, proto);
}
