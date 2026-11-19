---
title: TinyGo
description: Build Go code into small, fast WASM modules for your app with TinyGo.
---

# TinyGo

[TinyGo](https://tinygo.org) compiles Go for small targets. The same source as the [Go](go.md) summary builds to a 284 KB module instead of 4.3 MB. Its first call compiles in about 74 ms instead of a second, and later calls load it from the cache in 3 ms instead of 38, holding under 1 MB of machine code instead of 18 MB. It's the way to write Go for a module on a hot path.

## Set up

```bash
brew install tinygo
```

TinyGo supports a range of Go versions (TinyGo 0.41: Go 1.19 to 1.26) and uses the `go` on your `PATH`. With a newer Go installed, TinyGo stops with `requires go version 1.19 through 1.26`. The go command can fetch a supported one for it:

```bash
export PATH="$(GOTOOLCHAIN=go1.26.0 go env GOROOT)/bin:$PATH"
tinygo version
```

## The module

[`example/summary/tinygo`](../../example/summary/tinygo/main.go) is the Go summary, unchanged: `encoding/json`, `os.Args`, stdin and stdout work as they do in Go. Its manifest says `"language": "TinyGo"`, since a TinyGo module doesn't say it itself.

## Build

```bash
tinygo build -target=wasip1 -opt=z -no-debug -scheduler=none -o pb_hooks/summary.pb.wasm .
```

- `-opt=z`: smallest code.
- `-no-debug`: no debug information.
- `-scheduler=none`: no goroutine scheduler. A module is one call that reads, computes and exits, so it rarely needs one, and leaving it out takes the summary from 481 KB to 284 KB. A program that starts a goroutine doesn't build with it; leave the flag off then.

## Call it

The page code is the same as for Go:

```ts
const { title, body } = ctx.req.body()
const out = await $wasm.run.async<{ text: string }>("summary", { title, body })
```

## What's different from Go

- **Most of the standard library works**, `encoding/json`, `strings`, `math`, `sort`, `time`, `crypto/*` and `encoding/*` included. Packages that lean hard on reflection or on the operating system may not build; [TinyGo's package list](https://tinygo.org/docs/reference/lang-support/stdlib/) says which.
- **Memory starts small:** a TinyGo module starts with 128 KB and grows as it needs, where a Go one starts with 3 MB.
- **Third-party packages** build when they stay inside what TinyGo supports. Try the build: TinyGo says which import it can't compile.

[Keeping state in JS](../guides/state.md) builds a second TinyGo module, with two functions and the state held by the app.
