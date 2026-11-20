---
title: "PocketOne alpha.10: PocketPages fixes"
description: PocketOne v0.0.1-alpha.10 fixes ctx.pb() writes and require() at request time in PocketPages.
date: 2026-11-20
author: Ben Allfree
tags: release
---

Alpha.10 fixes two PocketPages bugs that broke ordinary apps.

**`ctx.pb()` writes work.** Every `create`, `update` or other call with a body threw `TypeError: operand 'prototype' property is not an object`. The PocketBase JS SDK checks `value instanceof FormData` on every request with a body, and in PocketOne's JS engine Go constructors like `FormData`, `Record` and `Collection` had no `prototype`, so `instanceof` threw. They now behave as they did under goja: each has a `prototype`, and `new X()` is an instance of `X`. Validation and rule failures from `ctx.pb()` come back as `ClientResponseError`, as the SDK documents.

**`require()` works on every request.** A `require()` inside a handler, or any function that runs at request time, used the request that first loaded the module. From the second request on it failed with `require("x"): not found`. It now resolves and loads files on its own.

Upgrade with `brew upgrade pocketone` or `pocketone upgrade`.

[Release notes](https://github.com/pocketonehq/pocketone/releases/tag/v0.0.1-alpha.10)
