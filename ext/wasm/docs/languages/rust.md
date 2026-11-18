---
title: Rust
description: Build WASM modules for pb_hooks from Rust, and use crates from crates.io in them.
---

# Rust

Rust makes small, fast modules (the summary is 98 KB: about 33 ms for its first call, which compiles it, and 1.3 ms for later ones), and most crates that only compute build for WASI as they are. It's the best choice when you need a library: a parser, a codec, a format. The [recipes](../recipes/index.md) are all Rust for that reason.

## Set up

```bash
rustup target add wasm32-wasip1
```

## The module

[`example/summary/rust`](../../example/summary/rust/src/main.rs) is a binary crate with serde for JSON:

```rust
fn fail(code: &str, message: &str) -> ! {
    eprint!("{}", serde_json::json!({ "code": code, "message": message }));
    std::process::exit(1)
}

fn main() {
    match std::env::args().nth(1).as_deref() {
        Some("--describe") => {
            print!("{}", include_str!("../manifest.json"));
            return;
        }
        Some("run") => {}
        other => fail("no_such_function", other.unwrap_or("")),
    }
    let post: Post = serde_json::from_reader(std::io::stdin())
        .unwrap_or_else(|e| fail("bad_input", &format!("the input isn't a post: {e}")));
    // ... the summary
    serde_json::to_writer(std::io::stdout(), &Summary { text, words: words.len() }).unwrap();
}
```

Keep the release profile small:

```toml
[profile.release]
opt-level = "z"
lto = true
strip = true
codegen-units = 1
```

## Build

```bash
cargo build --release --target wasm32-wasip1
cp target/wasm32-wasip1/release/summary.wasm pb_hooks/summary.pb.wasm
```

`strip = true` also drops what tells `$wasm.modules()` the module is Rust, so the manifest says `"language": "Rust"`.

## Call it

```js
const out = await $wasm.run.async("summary", { title: e.record.get("title"), body: e.record.get("body") })
```

## Crates

A crate works when it only computes. One that needs threads, sockets, files or a C library built for another target won't build or won't run: `rayon`, `tokio`'s runtime, `reqwest`, `openssl`. Turn off default features you don't need (`image = { version = "0.25", default-features = false, features = ["png"] }`); it keeps the module small and leaves out code that can't build for WASI. A panic ends the call as `crashed`; return an error with `fail` instead when the input is wrong.

Don't build for `wasm32-wasip2` or with `cargo component`: those make a component, which doesn't run here.
