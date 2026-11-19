; The summary module written by hand in LLVM IR, the language compilers hand to LLVM. clang turns it into a WASI
; program and links wasi-libc, so it calls read, write and malloc like C does.
;
;   stdin:  a text, as is (the hook passes a string: $wasm.run.async("summary", body))
;   stdout: {"text": "<first 12 words> …", "words": <word count>}
;
; Build (Zig 0.17, whose clang reads .ll files like .c ones):
;
;   zig cc -target wasm32-wasi -Os -s -Wl,-z,stack-size=1048576 -o summary.pb.wasm summary.ll
;
; Locals live in allocas (stack slots) to keep it readable; -Os turns them into registers.
target datalayout = "e-m:e-p:32:32-p10:8:8-p20:8:8-i64:64-i128:128-n32:64-S128-ni:1:10:20"
target triple = "wasm32-unknown-wasip1-unknown"

; what `summary --describe` prints: manifest.json, compacted (the tests check they match)
@manifest = private constant [334 x i8] c"{\22description\22:\22Summarizes a text: its first words, and a word count. Written by hand in LLVM IR\22,\22functions\22:[\22run\22],\22language\22:\22LLVM IR\22,\22repository\22:\22https://github.com/pockethost/pocketone/tree/main/ext/wasm/example/summary/llvm\22,\22examples\22:[{\22function\22:\22run\22,\22input\22:\22one two three\22,\22output\22:{\22text\22:\22one two three\22,\22words\22:3}}]}"
@describe = private constant [11 x i8] c"--describe\00"
@run = private constant [4 x i8] c"run\00"
@err_nsf = private constant [48 x i8] c"{\22code\22:\22no_such_function\22,\22message\22:\22only run\22}"
@err_empty = private constant [55 x i8] c"{\22code\22:\22empty_body\22,\22message\22:\22the text has no words\22}"
@err_read = private constant [52 x i8] c"{\22code\22:\22bad_input\22,\22message\22:\22stdin can't be read\22}"
@prefix = private constant [9 x i8] c"{\22text\22:\22"
@middle = private constant [10 x i8] c"\22,\22words\22:"
@ellipsis = private constant [4 x i8] c" \E2\80\A6"
@hex = private constant [16 x i8] c"0123456789abcdef"

declare i32 @read(i32, ptr, i32)
declare i32 @write(i32, ptr, i32)
declare ptr @malloc(i32)
declare ptr @realloc(ptr, i32)
declare i32 @strcmp(ptr, ptr)
declare void @exit(i32) noreturn

; write all of [ptr, ptr+len) to fd
define internal void @write_all(i32 %fd, ptr %ptr, i32 %len) {
entry:
  br label %loop
loop:
  %p = phi ptr [ %ptr, %entry ], [ %p.next, %more ]
  %left = phi i32 [ %len, %entry ], [ %left.next, %more ]
  %done = icmp sle i32 %left, 0
  br i1 %done, label %exit, label %write
write:
  %n = call i32 @write(i32 %fd, ptr %p, i32 %left)
  %failed = icmp slt i32 %n, 0
  br i1 %failed, label %exit, label %more
more:
  %p.next = getelementptr i8, ptr %p, i32 %n
  %left.next = sub i32 %left, %n
  br label %loop
exit:
  ret void
}

; end the program with an error pb_hooks can read (err.code / err.message)
define internal void @fail(ptr %msg, i32 %len) noreturn {
  call void @write_all(i32 2, ptr %msg, i32 %len)
  call void @exit(i32 1)
  unreachable
}

; space, tab, newline, vertical tab, form feed, carriage return
define internal i1 @space(i8 %c) {
  %is.sp = icmp eq i8 %c, 32
  %d = sub i8 %c, 9
  %is.ctl = icmp ult i8 %d, 5
  %r = or i1 %is.sp, %is.ctl
  ret i1 %r
}

; main(argc, argv): on WASI, C compilers name it __main_argc_argv, and wasi-libc's _start calls it by that name
define i32 @__main_argc_argv(i32 %argc, ptr %argv) {
entry:
  %buf = alloca ptr
  %cap = alloca i32
  %len = alloca i32
  %i = alloca i32
  %words = alloca i32
  %short = alloca i32
  %p = alloca ptr
  %first = alloca i1
  %digits = alloca [12 x i8]
  %d = alloca i32

  ; the function asked for: argv[1]
  %has.fn = icmp sgt i32 %argc, 1
  br i1 %has.fn, label %fn, label %no.such
fn:
  %fn.slot = getelementptr ptr, ptr %argv, i32 1
  %fn.name = load ptr, ptr %fn.slot
  %cmp.d = call i32 @strcmp(ptr %fn.name, ptr @describe)
  %is.d = icmp eq i32 %cmp.d, 0
  br i1 %is.d, label %describe, label %not.d
describe:
  call void @write_all(i32 1, ptr @manifest, i32 334)
  ret i32 0
not.d:
  %cmp.r = call i32 @strcmp(ptr %fn.name, ptr @run)
  %is.r = icmp eq i32 %cmp.r, 0
  br i1 %is.r, label %start, label %no.such
no.such:
  call void @fail(ptr @err_nsf, i32 48)
  unreachable

  ; read all of stdin, doubling the buffer as it fills
start:
  %b0 = call ptr @malloc(i32 65536)
  store ptr %b0, ptr %buf
  store i32 65536, ptr %cap
  store i32 0, ptr %len
  br label %read
read:
  %r.cap = load i32, ptr %cap
  %r.len = load i32, ptr %len
  %room = sub i32 %r.cap, %r.len
  %full = icmp ult i32 %room, 4096
  br i1 %full, label %grow, label %read.some
grow:
  %cap2 = shl i32 %r.cap, 1
  %old = load ptr, ptr %buf
  %new = call ptr @realloc(ptr %old, i32 %cap2)
  store ptr %new, ptr %buf
  store i32 %cap2, ptr %cap
  br label %read
read.some:
  %rb = load ptr, ptr %buf
  %at = getelementptr i8, ptr %rb, i32 %r.len
  %n = call i32 @read(i32 0, ptr %at, i32 %room)
  %bad = icmp slt i32 %n, 0
  br i1 %bad, label %read.failed, label %read.ok
read.failed:
  call void @fail(ptr @err_read, i32 52)
  unreachable
read.ok:
  %eof = icmp eq i32 %n, 0
  %len2 = add i32 %r.len, %n
  store i32 %len2, ptr %len
  br i1 %eof, label %count, label %read

  ; count the words, and remember where the 12th one ends
count:
  %text = load ptr, ptr %buf
  %total = load i32, ptr %len
  store i32 0, ptr %i
  store i32 0, ptr %words
  store i32 0, ptr %short
  br label %c.skip
c.skip:
  %cs.i = load i32, ptr %i
  %cs.end = icmp uge i32 %cs.i, %total
  br i1 %cs.end, label %counted, label %c.skip.test
c.skip.test:
  %cs.at = getelementptr i8, ptr %text, i32 %cs.i
  %cs.c = load i8, ptr %cs.at
  %cs.sp = call i1 @space(i8 %cs.c)
  %cs.i2 = add i32 %cs.i, 1
  br i1 %cs.sp, label %c.skip.next, label %c.word
c.skip.next:
  store i32 %cs.i2, ptr %i
  br label %c.skip
c.word:
  %cw.i = load i32, ptr %i
  %cw.end = icmp uge i32 %cw.i, %total
  br i1 %cw.end, label %c.word.done, label %c.word.test
c.word.test:
  %cw.at = getelementptr i8, ptr %text, i32 %cw.i
  %cw.c = load i8, ptr %cw.at
  %cw.sp = call i1 @space(i8 %cw.c)
  br i1 %cw.sp, label %c.word.done, label %c.word.next
c.word.next:
  %cw.i2 = add i32 %cw.i, 1
  store i32 %cw.i2, ptr %i
  br label %c.word
c.word.done:
  %w = load i32, ptr %words
  %w2 = add i32 %w, 1
  store i32 %w2, ptr %words
  %in.short = icmp ule i32 %w2, 12
  br i1 %in.short, label %c.mark, label %c.skip
c.mark:
  %cm.i = load i32, ptr %i
  store i32 %cm.i, ptr %short
  br label %c.skip
counted:
  %nwords = load i32, ptr %words
  %none = icmp eq i32 %nwords, 0
  br i1 %none, label %empty, label %output
empty:
  call void @fail(ptr @err_empty, i32 55)
  unreachable

  ; {"text":" then the first 12 words, one space between them, as a JSON string
output:
  %short.end = load i32, ptr %short
  %worst = mul i32 %short.end, 6
  %size = add i32 %worst, 64
  %out = call ptr @malloc(i32 %size)
  call void @llvm.memcpy.p0.p0.i32(ptr %out, ptr @prefix, i32 9, i1 false)
  %p0 = getelementptr i8, ptr %out, i32 9
  store ptr %p0, ptr %p
  store i32 0, ptr %i
  store i1 true, ptr %first
  br label %o.skip
o.skip:
  %os.i = load i32, ptr %i
  %os.end = icmp uge i32 %os.i, %short.end
  br i1 %os.end, label %o.tail, label %o.skip.test
o.skip.test:
  %os.at = getelementptr i8, ptr %text, i32 %os.i
  %os.c = load i8, ptr %os.at
  %os.sp = call i1 @space(i8 %os.c)
  br i1 %os.sp, label %o.skip.next, label %o.word
o.skip.next:
  %os.i2 = add i32 %os.i, 1
  store i32 %os.i2, ptr %i
  br label %o.skip
o.word:
  %f = load i1, ptr %first
  store i1 false, ptr %first
  br i1 %f, label %o.char, label %o.space
o.space:
  %sp.p = load ptr, ptr %p
  store i8 32, ptr %sp.p
  %sp.p2 = getelementptr i8, ptr %sp.p, i32 1
  store ptr %sp.p2, ptr %p
  br label %o.char
o.char:
  %oc.i = load i32, ptr %i
  %oc.end = icmp uge i32 %oc.i, %short.end
  br i1 %oc.end, label %o.skip, label %o.char.load
o.char.load:
  %oc.at = getelementptr i8, ptr %text, i32 %oc.i
  %c = load i8, ptr %oc.at
  %c.sp = call i1 @space(i8 %c)
  br i1 %c.sp, label %o.skip, label %o.escape
o.escape:
  %e.p = load ptr, ptr %p
  %is.q = icmp eq i8 %c, 34
  %is.bs = icmp eq i8 %c, 92
  %q.or.bs = or i1 %is.q, %is.bs
  br i1 %q.or.bs, label %o.backslash, label %o.ctl.test
o.backslash:                                   ; \" and \\
  store i8 92, ptr %e.p
  %bs.p1 = getelementptr i8, ptr %e.p, i32 1
  store i8 %c, ptr %bs.p1
  %bs.p2 = getelementptr i8, ptr %e.p, i32 2
  store ptr %bs.p2, ptr %p
  br label %o.char.next
o.ctl.test:
  %is.ctl = icmp ult i8 %c, 32
  br i1 %is.ctl, label %o.ctl, label %o.plain
o.ctl:                                          ; a control character: \u00XX
  store i32 808482140, ptr %e.p, align 1        ; the bytes \ u 0 0
  %hi = lshr i8 %c, 4
  %lo = and i8 %c, 15
  %hi.i = zext i8 %hi to i32
  %lo.i = zext i8 %lo to i32
  %hi.at = getelementptr i8, ptr @hex, i32 %hi.i
  %lo.at = getelementptr i8, ptr @hex, i32 %lo.i
  %hi.c = load i8, ptr %hi.at
  %lo.c = load i8, ptr %lo.at
  %ctl.p4 = getelementptr i8, ptr %e.p, i32 4
  store i8 %hi.c, ptr %ctl.p4
  %ctl.p5 = getelementptr i8, ptr %e.p, i32 5
  store i8 %lo.c, ptr %ctl.p5
  %ctl.p6 = getelementptr i8, ptr %e.p, i32 6
  store ptr %ctl.p6, ptr %p
  br label %o.char.next
o.plain:
  store i8 %c, ptr %e.p
  %pl.p1 = getelementptr i8, ptr %e.p, i32 1
  store ptr %pl.p1, ptr %p
  br label %o.char.next
o.char.next:
  %oc.i2 = add i32 %oc.i, 1
  store i32 %oc.i2, ptr %i
  br label %o.char

  ; " …" when there's more, then ","words":<count>}
o.tail:
  %more = icmp ugt i32 %nwords, 12
  br i1 %more, label %o.ellipsis, label %o.count
o.ellipsis:
  %el.p = load ptr, ptr %p
  call void @llvm.memcpy.p0.p0.i32(ptr %el.p, ptr @ellipsis, i32 4, i1 false)
  %el.p2 = getelementptr i8, ptr %el.p, i32 4
  store ptr %el.p2, ptr %p
  br label %o.count
o.count:
  %mid.p = load ptr, ptr %p
  call void @llvm.memcpy.p0.p0.i32(ptr %mid.p, ptr @middle, i32 10, i1 false)
  %mid.p2 = getelementptr i8, ptr %mid.p, i32 10
  store ptr %mid.p2, ptr %p
  store i32 12, ptr %d                          ; the digits, written backwards into %digits
  store i32 %nwords, ptr %words
  br label %digit
digit:
  %dg.d = load i32, ptr %d
  %dg.d2 = sub i32 %dg.d, 1
  store i32 %dg.d2, ptr %d
  %dg.w = load i32, ptr %words
  %dg.rem = urem i32 %dg.w, 10
  %dg.quot = udiv i32 %dg.w, 10
  store i32 %dg.quot, ptr %words
  %dg.rem8 = trunc i32 %dg.rem to i8
  %dg.c = add i8 %dg.rem8, 48
  %dg.at = getelementptr i8, ptr %digits, i32 %dg.d2
  store i8 %dg.c, ptr %dg.at
  %dg.more = icmp ne i32 %dg.quot, 0
  br i1 %dg.more, label %digit, label %finish
finish:
  %fi.d = load i32, ptr %d
  %fi.from = getelementptr i8, ptr %digits, i32 %fi.d
  %fi.n = sub i32 12, %fi.d
  %fi.p = load ptr, ptr %p
  call void @llvm.memcpy.p0.p0.i32(ptr %fi.p, ptr %fi.from, i32 %fi.n, i1 false)
  %fi.p2 = getelementptr i8, ptr %fi.p, i32 %fi.n
  store i8 125, ptr %fi.p2                      ; }
  %fi.end = getelementptr i8, ptr %fi.p2, i32 1
  %out.end = ptrtoint ptr %fi.end to i32
  %out.start = ptrtoint ptr %out to i32
  %out.len = sub i32 %out.end, %out.start
  call void @write_all(i32 1, ptr %out, i32 %out.len)
  ret i32 0
}

declare void @llvm.memcpy.p0.p0.i32(ptr, ptr, i32, i1)
