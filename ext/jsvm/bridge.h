#ifndef ASYNCVM_BRIDGE_H
#define ASYNCVM_BRIDGE_H

#include <stdint.h>
#include <stdlib.h>
#include "quickjs.h"

// One app's VM: a QuickJS runtime + context owned by one locked OS thread.
typedef struct AVM {
	JSRuntime *rt;
	JSContext *ctx;
	uintptr_t id;          // cgo.Handle of the Go *VM
	int interrupt;         // set from any goroutine (atomic)
	int64_t deadline_ns;   // CLOCK_MONOTONIC; 0 = no CPU budget
	int64_t wall_ns;       // hard wall-clock end of the current top-level task, Go time included; 0 = none
	uint64_t thread;       // pthread_self of the owner
	JSClassID obj_class;   // GoObject: exotic get/set/has/keys
	JSClassID fn_class;    // GoFunc: callable, constructor
	int64_t interrupts;    // how many times the handler stopped JS
} AVM;

AVM *avm_new(uintptr_t id, size_t mem_limit, size_t stack_size);
void avm_free(AVM *a);
int avm_on_thread(AVM *a);
void avm_set_interrupt(AVM *a, int v);
int64_t avm_now_ns(void);

JSValue avm_new_object(AVM *a, uint32_t handle);
JSValue avm_new_func(AVM *a, uint32_t handle, int ctor);
// handle of a GoObject/GoFunc, 0 if not one.
uint32_t avm_handle_of(AVM *a, JSValue v);

// value helpers (macros/inlines in quickjs.h)
int avm_tag(JSValue v);
JSValue avm_undefined(void);
JSValue avm_null(void);
JSValue avm_bool(int b);
JSValue avm_float(double d);
JSValue avm_int(JSContext *ctx, int64_t i);
JSValue avm_dup(JSContext *ctx, JSValue v);
int avm_is_exception(JSValue v);
int avm_is_function(JSContext *ctx, JSValue v);
JSValue avm_throw(JSContext *ctx, const char *msg, size_t n);
JSValue avm_throw_type(JSContext *ctx, const char *msg);
JSValue avm_eval(JSContext *ctx, const char *code, size_t n, const char *file);
int avm_run_jobs(AVM *a); // run queued promise jobs; returns count, -1 on a job exception
JSValue avm_array_get(JSContext *ctx, JSValue arr, uint32_t i);
int64_t avm_length(JSContext *ctx, JSValue v);
int avm_is_array(JSValue v);
JSValue avm_new_array(JSContext *ctx);
void avm_array_push(JSContext *ctx, JSValue arr, uint32_t i, JSValue v);
// object keys as a JS array of strings (own enumerable)
JSValue avm_object_keys(JSContext *ctx, JSValue o);
JSValue avm_get_str(JSContext *ctx, JSValue o, const char *k);
void avm_set_str(JSContext *ctx, JSValue o, const char *k, JSValue v);
JSValue avm_call(JSContext *ctx, JSValue fn, JSValue this_val, int argc, JSValue *argv);
JSValue avm_new_promise(JSContext *ctx, JSValue *funcs);
int avm_promise_state(JSContext *ctx, JSValue p);
JSValue avm_promise_result(JSContext *ctx, JSValue p);
int avm_is_promise(JSValue v);
size_t avm_mem_used(AVM *a);


void avm_set_protos(AVM *a, JSValue obj_proto, JSValue fn_proto);
JSValue avm_parse_json(JSContext *ctx, const char *buf, size_t n);
void avm_define_hidden(JSContext *ctx, JSValue obj, const char *name, JSValue v);
JSValue avm_eval_flags(JSContext *ctx, const char *code, size_t n, const char *file, int flags);
int avm_set_proto(JSContext *ctx, JSValue obj, JSValue proto);
#endif
