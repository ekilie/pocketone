# syntax=docker/dockerfile:1
#
# The build environment of every example module (example/<folder>/<name>.pb.wasm): one stage per language's
# toolchain (rust, zig for Zig, C and LLVM IR, tinygo, go, wabt for WebAssembly text, assemblyscript), pinned so a rebuild anywhere
# gives the same bytes. The tests build each stage and run the module's build in it: go test -run TestPrebuiltExamples
# -update rebuilds the kept files, go test -run TestPrebuiltMatchSources -rebuild checks them.
#
# To move to a newer compiler: change its version here (and the digest: docker buildx imagetools inspect <image>),
# rebuild every module with -update, and commit the Dockerfile and the modules together.

# Rust 1.100.0, with the wasm32-wasip1 target
FROM rust:1.100.0-slim-bookworm@sha256:7c7d4eb4d05b7781187a327809466cfddf29753f6defdc3d19a0e291951b7a37 AS rust
RUN rustup target add wasm32-wasip1
WORKDIR /src

# Zig 0.17.0, which also builds the C and LLVM IR examples (zig cc brings clang and wasi-libc)
FROM debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc AS zig
ARG TARGETARCH
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl xz-utils \
 && rm -rf /var/lib/apt/lists/*
RUN case "$TARGETARCH" in \
      amd64) arch=x86_64;  sum=1cbe9df9f27e6b78d14ccbca43b6703a404ef79ef1c463de901d7f088d4e2026 ;; \
      arm64) arch=aarch64; sum=9e8d11661d4ae3bd57702a3832781e23ad151dde5798e16a5ccd503f65234ff8 ;; \
      *) echo "no Zig for $TARGETARCH" >&2; exit 1 ;; \
    esac \
 && curl -fsSL -o /tmp/zig.tar.xz "https://ziglang.org/download/0.17.0/zig-$arch-linux-0.17.0.tar.xz" \
 && echo "$sum  /tmp/zig.tar.xz" | sha256sum -c - \
 && mkdir /opt/zig && tar -xJf /tmp/zig.tar.xz -C /opt/zig --strip-components=1 && rm /tmp/zig.tar.xz
ENV PATH=/opt/zig:$PATH ZIG_GLOBAL_CACHE_DIR=/tmp/zig-cache
WORKDIR /src

# TinyGo 0.41.1, with the Go it ships (1.26)
FROM tinygo/tinygo:0.41.1@sha256:b216f534ddbf277444407b014a3328b5c1ade403cc397f3ab48a14789bf99d0e AS tinygo
ENV HOME=/tmp
WORKDIR /src

# Go 1.27.4 (GOOS=wasip1); -trimpath and no VCS stamp, so a module holds no path or commit of the build
FROM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS go
ENV GOTOOLCHAIN=local GOOS=wasip1 GOARCH=wasm GOFLAGS="-trimpath -buildvcs=false" GOCACHE=/tmp/go-cache HOME=/tmp
WORKDIR /src

# WABT 1.0.41 (wat2wasm), for the example written in WebAssembly text
FROM debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc AS wabt
ARG TARGETARCH
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*
RUN case "$TARGETARCH" in \
      amd64) arch=x64;   sum=83f8122e924745fcd70636e3594bc01c4c47f2d4c8f3c63b5d70d3f83a482677 ;; \
      arm64) arch=arm64; sum=5e35416ee8725dc7cc0572e4392a8117cbf008b0e34c0db65c75506b0299cdbf ;; \
      *) echo "no WABT for $TARGETARCH" >&2; exit 1 ;; \
    esac \
 && curl -fsSL -o /tmp/wabt.tar.gz "https://github.com/WebAssembly/wabt/releases/download/1.0.41/wabt-1.0.41-linux-$arch.tar.gz" \
 && echo "$sum  /tmp/wabt.tar.gz" | sha256sum -c - \
 && mkdir /opt/wabt && tar -xzf /tmp/wabt.tar.gz -C /opt/wabt --strip-components=1 && rm /tmp/wabt.tar.gz
ENV PATH=/opt/wabt/bin:$PATH
WORKDIR /src

# AssemblyScript 0.28.20 with @assemblyscript/wasi-shim 0.1.0, on Node 24.22.3. Installed here, at those versions, so a
# build needs no network: it runs on a copy of the source with these node_modules linked in.
FROM node:24.22.3-bookworm-slim@sha256:dc26ae92c9fb15cf6b1d63e20f0944b2efe0f115b20e00be48fb23fceb061c54 AS assemblyscript
RUN mkdir /opt/as && cd /opt/as && npm init -y >/dev/null \
 && npm install --no-audit --no-fund --save-exact assemblyscript@0.28.20 @assemblyscript/wasi-shim@0.1.0
ENV HOME=/tmp
WORKDIR /src
