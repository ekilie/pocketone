---
title: LLVM IR
description: Write a WASM module for pb_hooks in LLVM IR, the assembly language compilers hand to LLVM, and let clang build it.
---

# LLVM IR

LLVM IR is the language Clang, Rust, Zig and Swift translate your code into before LLVM turns it into machine code, or here, WebAssembly. It reads like a portable assembly language: typed values, basic blocks, branches. Written by hand, it gets LLVM's optimizer and wasi-libc for free. The summary example is 3.8 KB; its first call compiles in about 7.5 ms, later calls load it from the cache in half a millisecond.

Like [WAT](wat.md), it's here for fun and to show what compilers produce. Seeing your C or Rust as IR is the useful part: `clang -S -emit-llvm` or `rustc --emit=llvm-ir`.

## Set up

Nothing new if you have Zig: `zig cc` is clang, and it reads `.ll` files as well as `.c` ones.

```bash
brew install zig
```

## The module

[`example/summary/llvm`](../../example/summary/llvm/summary.ll) does what the [C](c.md) summary does, in IR, and calls wasi-libc's `read`, `write`, `malloc` and `exit` like C would:

```llvm
target datalayout = "e-m:e-p:32:32-p10:8:8-p20:8:8-i64:64-i128:128-n32:64-S128-ni:1:10:20"
target triple = "wasm32-unknown-wasip1-unknown"

@err_nsf = private constant [48 x i8] c"{\22code\22:\22no_such_function\22,\22message\22:\22only run\22}"

declare i32 @write(i32, ptr, i32)
declare void @exit(i32) noreturn

; space, tab, newline, vertical tab, form feed, carriage return
define internal i1 @space(i8 %c) {
  %is.sp = icmp eq i8 %c, 32
  %d = sub i8 %c, 9
  %is.ctl = icmp ult i8 %d, 5
  %r = or i1 %is.sp, %is.ctl
  ret i1 %r
}

; main(argc, argv): on WASI, C compilers name it __main_argc_argv
define i32 @__main_argc_argv(i32 %argc, ptr %argv) {
  ; ...
}
```

Two things hand-written IR has to get right that a compiler would do for you:

- **The datalayout and triple** must be wasm32-WASI's. Get them from clang: `zig cc -target wasm32-wasi -S -emit-llvm -o - x.c | head`.
- **`main` is called `__main_argc_argv`.** For WASI, clang renames a C `main(argc, argv)`, and wasi-libc's `_start` calls it by that name. Name it `main` and it isn't the function libc calls with the arguments.

The example keeps its locals in `alloca` stack slots, which is easier to write than SSA with `phi` nodes; `-Os` turns them into registers.

## Build

```bash
zig cc -target wasm32-wasi -Os -s -Wl,-z,stack-size=1048576 -o pb_hooks/summary.pb.wasm summary.ll
```

The flags are the [C](c.md) ones: smallest code, no symbols, a 1 MB stack instead of 16 MB.

## Call it

```js
const out = await $wasm.run.async("summary", e.record.get("body")) // a string goes in as it is
```
