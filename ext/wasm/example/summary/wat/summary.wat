;; The summary module written by hand in WebAssembly text (WAT): no compiler, no libc, only WASI calls.
;;
;;   stdin:  a text, as is (the hook passes a string: $wasm.run.async("summary", body))
;;   stdout: {"text": "<first 12 words> …", "words": <word count>}
;;
;; Build (WABT 1.0.41):  wat2wasm summary.wat -o summary.pb.wasm
;;
;; Memory:  0 scratch (iovec, counts)  64 strings  1024 argv  1280 argv bytes  4096 manifest  8192 digits
;;          65536 and up: the input, then the output (memory grows as they need)
(module
  (import "wasi_snapshot_preview1" "args_sizes_get" (func $args_sizes_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "args_get" (func $args_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "fd_read" (func $fd_read (param i32 i32 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "fd_write" (func $fd_write (param i32 i32 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "proc_exit" (func $proc_exit (param i32)))

  (memory (export "memory") 2)

  (data (i32.const 64) "--describe")                                                        ;; 10 bytes
  (data (i32.const 80) "run")                                                               ;; 3
  (data (i32.const 96) "{\22code\22:\22empty_body\22,\22message\22:\22the text has no words\22}") ;; 55
  (data (i32.const 160) "{\22code\22:\22no_such_function\22,\22message\22:\22only run\22}")       ;; 48
  (data (i32.const 224) "{\22code\22:\22bad_input\22,\22message\22:\22stdin can't be read\22}")   ;; 52
  (data (i32.const 288) "{\22text\22:\22")                                                   ;; 9
  (data (i32.const 304) "\22,\22words\22:")                                                  ;; 10
  (data (i32.const 320) " \e2\80\a6")                                                        ;; 4: " …"
  (data (i32.const 336) "0123456789abcdef")
  ;; what `summary --describe` prints: manifest.json, compacted (the tests check they match)
  (data (i32.const 4096) "{\22description\22:\22Summarizes a text: its first words, and a word count. Written by hand in WebAssembly text\22,\22functions\22:[\22run\22],\22language\22:\22WAT\22,\22repository\22:\22https://github.com/pockethost/pocketone/tree/main/ext/wasm/example/summary/wat\22,\22examples\22:[{\22function\22:\22run\22,\22input\22:\22one two three\22,\22output\22:{\22text\22:\22one two three\22,\22words\22:3}}]}")

  ;; write all of [ptr, ptr+len) to fd
  (func $write (param $fd i32) (param $ptr i32) (param $len i32)
    (block $done
      (loop $more
        (br_if $done (i32.eqz (local.get $len)))
        (i32.store (i32.const 0) (local.get $ptr))
        (i32.store (i32.const 4) (local.get $len))
        (br_if $done (call $fd_write (local.get $fd) (i32.const 0) (i32.const 1) (i32.const 8)))
        (local.set $ptr (i32.add (local.get $ptr) (i32.load (i32.const 8))))
        (local.set $len (i32.sub (local.get $len) (i32.load (i32.const 8))))
        (br $more))))

  ;; end the program with an error pb_hooks can read (err.code / err.message)
  (func $fail (param $ptr i32) (param $len i32)
    (call $write (i32.const 2) (local.get $ptr) (local.get $len))
    (call $proc_exit (i32.const 1)))

  ;; are the len bytes at a and b the same?
  (func $eq (param $a i32) (param $b i32) (param $len i32) (result i32)
    (block $no
      (loop $next
        (if (i32.eqz (local.get $len)) (then (return (i32.const 1))))
        (br_if $no (i32.ne (i32.load8_u (local.get $a)) (i32.load8_u (local.get $b))))
        (local.set $a (i32.add (local.get $a) (i32.const 1)))
        (local.set $b (i32.add (local.get $b) (i32.const 1)))
        (local.set $len (i32.sub (local.get $len) (i32.const 1)))
        (br $next)))
    (i32.const 0))

  ;; space, tab, newline, vertical tab, form feed, carriage return
  (func $space (param $c i32) (result i32)
    (i32.or (i32.eq (local.get $c) (i32.const 32))
            (i32.lt_u (i32.sub (local.get $c) (i32.const 9)) (i32.const 5))))

  ;; make sure memory reaches the address end
  (func $reach (param $end i32)
    (local $pages i32)
    (local.set $pages (i32.sub (i32.shr_u (i32.add (local.get $end) (i32.const 65535)) (i32.const 16)) (memory.size)))
    (if (i32.gt_s (local.get $pages) (i32.const 0))
      (then (if (i32.eq (memory.grow (local.get $pages)) (i32.const -1))
        (then (unreachable))))))  ;; out of memory: the call fails as crashed

  (func $main (export "_start")
    (local $fn i32) (local $fnlen i32)
    (local $len i32) (local $n i32) (local $i i32) (local $c i32)
    (local $words i32) (local $shortEnd i32) (local $out i32) (local $p i32) (local $first i32)

    ;; the function asked for: argv[1]
    (drop (call $args_sizes_get (i32.const 16) (i32.const 20)))
    (if (i32.or (i32.lt_u (i32.load (i32.const 16)) (i32.const 2))
                (i32.or (i32.gt_u (i32.load (i32.const 16)) (i32.const 64))
                        (i32.gt_u (i32.load (i32.const 20)) (i32.const 2816))))
      (then (call $fail (i32.const 160) (i32.const 48))))
    (drop (call $args_get (i32.const 1024) (i32.const 1280)))
    (local.set $fn (i32.load (i32.const 1028)))
    (local.set $fnlen (i32.const 0))
    (loop $strlen
      (if (i32.load8_u (i32.add (local.get $fn) (local.get $fnlen)))
        (then (local.set $fnlen (i32.add (local.get $fnlen) (i32.const 1))) (br $strlen))))

    ;; --describe prints the manifest; run is the only function
    (if (i32.and (i32.eq (local.get $fnlen) (i32.const 10)) (call $eq (local.get $fn) (i32.const 64) (i32.const 10)))
      (then (call $write (i32.const 1) (i32.const 4096) (i32.const 338)) (return)))
    (if (i32.eqz (i32.and (i32.eq (local.get $fnlen) (i32.const 3)) (call $eq (local.get $fn) (i32.const 80) (i32.const 3))))
      (then (call $fail (i32.const 160) (i32.const 48))))

    ;; read all of stdin to 65536, 64 KB at a time
    (block $eof
      (loop $read
        (call $reach (i32.add (i32.const 131072) (local.get $len)))
        (i32.store (i32.const 0) (i32.add (i32.const 65536) (local.get $len)))
        (i32.store (i32.const 4) (i32.const 65536))
        (if (call $fd_read (i32.const 0) (i32.const 0) (i32.const 1) (i32.const 8))
          (then (call $fail (i32.const 224) (i32.const 52))))
        (local.set $n (i32.load (i32.const 8)))
        (br_if $eof (i32.eqz (local.get $n)))
        (local.set $len (i32.add (local.get $len) (local.get $n)))
        (br $read)))

    ;; count the words, and remember where the 12th one ends
    (block $counted
      (loop $word
        (block $skipped
          (loop $skip
            (br_if $skipped (i32.ge_u (local.get $i) (local.get $len)))
            (br_if $skipped (i32.eqz (call $space (i32.load8_u (i32.add (i32.const 65536) (local.get $i))))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $skip)))
        (br_if $counted (i32.ge_u (local.get $i) (local.get $len)))
        (block $ended
          (loop $in
            (br_if $ended (i32.ge_u (local.get $i) (local.get $len)))
            (br_if $ended (call $space (i32.load8_u (i32.add (i32.const 65536) (local.get $i)))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $in)))
        (local.set $words (i32.add (local.get $words) (i32.const 1)))
        (if (i32.le_u (local.get $words) (i32.const 12)) (then (local.set $shortEnd (local.get $i))))
        (br $word)))
    (if (i32.eqz (local.get $words)) (then (call $fail (i32.const 96) (i32.const 55))))

    ;; the output goes after the input: at most 6 bytes per input byte (\u00XX), plus the rest
    (local.set $out (i32.add (i32.const 65536) (local.get $len)))
    (call $reach (i32.add (i32.add (local.get $out) (i32.mul (local.get $shortEnd) (i32.const 6))) (i32.const 64)))
    (memory.copy (local.get $out) (i32.const 288) (i32.const 9))
    (local.set $p (i32.add (local.get $out) (i32.const 9)))

    ;; the first 12 words, one space between them, as a JSON string
    (local.set $i (i32.const 0))
    (local.set $first (i32.const 1))
    (block $copied
      (loop $word
        (block $skipped
          (loop $skip
            (br_if $skipped (i32.ge_u (local.get $i) (local.get $shortEnd)))
            (br_if $skipped (i32.eqz (call $space (i32.load8_u (i32.add (i32.const 65536) (local.get $i))))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $skip)))
        (br_if $copied (i32.ge_u (local.get $i) (local.get $shortEnd)))
        (if (i32.eqz (local.get $first))
          (then (i32.store8 (local.get $p) (i32.const 32)) (local.set $p (i32.add (local.get $p) (i32.const 1)))))
        (local.set $first (i32.const 0))
        (block $ended
          (loop $in
            (br_if $ended (i32.ge_u (local.get $i) (local.get $shortEnd)))
            (local.set $c (i32.load8_u (i32.add (i32.const 65536) (local.get $i))))
            (br_if $ended (call $space (local.get $c)))
            (if (i32.or (i32.eq (local.get $c) (i32.const 34)) (i32.eq (local.get $c) (i32.const 92)))
              (then  ;; \" and \\
                (i32.store8 (local.get $p) (i32.const 92))
                (i32.store8 (i32.add (local.get $p) (i32.const 1)) (local.get $c))
                (local.set $p (i32.add (local.get $p) (i32.const 2))))
              (else (if (i32.lt_u (local.get $c) (i32.const 32))
                (then  ;; a control character: \u00XX
                  (i32.store (local.get $p) (i32.const 0x3030755c))
                  (i32.store8 (i32.add (local.get $p) (i32.const 4)) (i32.load8_u (i32.add (i32.const 336) (i32.shr_u (local.get $c) (i32.const 4)))))
                  (i32.store8 (i32.add (local.get $p) (i32.const 5)) (i32.load8_u (i32.add (i32.const 336) (i32.and (local.get $c) (i32.const 15)))))
                  (local.set $p (i32.add (local.get $p) (i32.const 6))))
                (else
                  (i32.store8 (local.get $p) (local.get $c))
                  (local.set $p (i32.add (local.get $p) (i32.const 1)))))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $in)))
        (br $word)))
    (if (i32.gt_u (local.get $words) (i32.const 12))
      (then (memory.copy (local.get $p) (i32.const 320) (i32.const 4)) (local.set $p (i32.add (local.get $p) (i32.const 4)))))

    ;; ","words":<count>}
    (memory.copy (local.get $p) (i32.const 304) (i32.const 10))
    (local.set $p (i32.add (local.get $p) (i32.const 10)))
    (local.set $i (i32.const 8212))  ;; the digits, written backwards from 8212
    (loop $digit
      (local.set $i (i32.sub (local.get $i) (i32.const 1)))
      (i32.store8 (local.get $i) (i32.add (i32.const 48) (i32.rem_u (local.get $words) (i32.const 10))))
      (local.set $words (i32.div_u (local.get $words) (i32.const 10)))
      (br_if $digit (local.get $words)))
    (memory.copy (local.get $p) (local.get $i) (i32.sub (i32.const 8212) (local.get $i)))
    (local.set $p (i32.add (local.get $p) (i32.sub (i32.const 8212) (local.get $i))))
    (i32.store8 (local.get $p) (i32.const 125))  ;; }
    (local.set $p (i32.add (local.get $p) (i32.const 1)))

    (call $write (i32.const 1) (local.get $out) (i32.sub (local.get $p) (local.get $out))))
)
