---
title: QuickJS hooks
description: PocketOne runs pb_hooks on QuickJS, a drop-in replacement for PocketBase's JS engine that adds imports, TypeScript, async handlers, shared globals and hard memory and time limits.
---

# QuickJS hooks

> **PocketBase:** hooks run on goja: CommonJS only, no async handlers, no state shared between handlers. **PocketOne:** the same JS API on QuickJS, with imports, TypeScript, async handlers, shared globals and hard limits.

PocketOne runs your `pb_hooks` and `pb_migrations` on [QuickJS](/docs/extensions/jsvm) instead of goja, the engine
stock PocketBase uses. It's the same JS API (`$app`, `routerAdd`, `onRecordCreate`, `cronAdd`, `$http`, …), so your
existing hooks run unchanged. On top of that you get:

- **Shared globals.** Top-level helpers and state are visible to every handler.
- **Imports.** ES modules with `import`/`export`, and `node_modules` packages.
- **TypeScript.** `.ts` hooks run directly.
- **Async.** Handlers can `await`, and the app keeps serving other requests while they wait.
- **Limits.** A runaway script stops with an error instead of taking the app down.

Each app gets one QuickJS VM. The engine is pure Go, so builds need no cgo, as with stock PocketBase.

This tutorial builds a small app that tries each of these. Every example below was run on `pocketone`, and the outputs
are real.

## Set up

Make a folder for the app, create a superuser, and start it:

```bash
mkdir -p qjs/pb_hooks/lib && cd qjs
pocketone superuser upsert admin@example.com password --dir=pb_data
pocketone serve --dir=pb_data
```

Leave it running. PocketOne watches `pb_hooks`, so each file you add below is picked up without a restart.

## 1. Your PocketBase hooks run unchanged

Here's a hook written for stock PocketBase, with `require()` inside the handler and the global `$app`:

```js
// pb_hooks/legacy.pb.js
routerAdd("GET", "/legacy", (e) => {
  const util = require(`${__hooks}/lib/util.cjs`)
  return e.json(200, { shout: util.shout("hello"), users: $app.countRecords("users") })
})
```

```js
// pb_hooks/lib/util.cjs
module.exports = { shout: (s) => s.toUpperCase() + "!" }
```

```bash
curl http://127.0.0.1:8090/legacy
# {"shout":"HELLO!","users":0}
```

A plain `.pb.js` file is still a CommonJS script, as in PocketBase. Nothing in it needs to change.

## 2. Top-level code is shared by every handler

On goja, each handler runs in a separate runtime from a pool, so it can't see the variables at the top of its own file.
That's why PocketBase hooks call `require()` inside every handler. On QuickJS, every hook file runs in the app's one
global scope:

```js
// pb_hooks/counter.pb.js
let hits = 0
function greet(name) {
  return `hi ${name}`
}

routerAdd("GET", "/hits", (e) => {
  hits++
  return e.json(200, { hits, msg: greet("there") })
})
```

```bash
curl http://127.0.0.1:8090/hits   # {"hits":1,"msg":"hi there"}
curl http://127.0.0.1:8090/hits   # {"hits":2,"msg":"hi there"}
```

The same file on goja (`pocketone serve --jsvm goja`) answers every request with a 400, because the handler can't see
`hits` or `greet`.

Globals live as long as the app's VM. A restart or a change to `pb_hooks` starts a fresh one (`hits` goes back to 0),
so keep anything that must last in the database. Use globals for caches, helpers and constants.

Hook files run before the app opens its database, as in PocketBase. A `$app` query at a file's top level fails, so to
fill a global from the database, do it in `onBootstrap`:

```js
let users = null
onBootstrap((e) => {
  e.next() // the database is open after this
  users = e.app.findCollectionByNameOrId("users")
})
```

## 3. Split your code into modules

Name a hook file `.pb.mjs` and it's an ES module, with `import` and `export`:

```js
// pb_hooks/lib/slug.mjs
export const slugify = (s) =>
  s.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "")
```

```js
// pb_hooks/modules.pb.mjs
import { slugify } from "./lib/slug.mjs"

routerAdd("GET", "/slug/{text}", (e) => {
  return e.json(200, { slug: slugify(e.request.pathValue("text")) })
})
```

```bash
curl "http://127.0.0.1:8090/slug/Hello%20World%21"
# {"slug":"hello-world"}
```

- `.mjs` files are ES modules; `.js` and `.cjs` files are CommonJS. Both kinds can load each other.
- `import` and `require()` find files the same way: relative paths first, then packages in `node_modules`.
- A module loads once per app, so every file that imports it shares the same instance and its state.

## 4. Write hooks in TypeScript

Name the file `.pb.ts` and write TypeScript:

```ts
// pb_hooks/typed.pb.ts
interface Total {
  count: number
  label: string
}

function total(count: number): Total {
  return { count, label: `${count} users` }
}

routerAdd("GET", "/typed", (e) => {
  return e.json(200, total(e.app.countRecords("users")))
})
```

```bash
curl http://127.0.0.1:8090/typed
# {"count":0,"label":"0 users"}
```

PocketOne removes the types when it loads the file, with no build step. An error names the line in your `.ts` file.
Types aren't checked at run time: run `tsc --noEmit` or rely on your editor. The app writes the whole JS API's types to
`pb_data/types.d.ts`, and the [JSVM page](/docs/extensions/jsvm#types) has a `tsconfig.json` that uses it.

## 5. Async handlers

A handler can be `async`. Every Go function has an `.async` twin that does its work outside the VM and returns a
promise:

```js
// pb_hooks/async.pb.js
routerAdd("GET", "/slow", async (e) => {
  await sleep.async(2000) // waits outside the VM
  const n = await e.app.countRecords.async("users")
  return e.json(200, { waited: "2s", users: n })
})
```

Call `/slow`, and call `/hits` while it waits:

```bash
curl http://127.0.0.1:8090/slow &
curl http://127.0.0.1:8090/hits
```

`/hits` answered after 0.35 s, and `/slow` after 2.06 s. The app went on serving requests while `/slow` waited.

Without `.async`, a call that waits (`$http.send`, `sleep`) still lets other requests run, but those requests nest on
one stack and wait on each other: under load, a plain `sleep(100)` served 19 requests a second where
`await sleep.async(100)` served 481 ([How it performs](#how-it-performs)). Use `.async` for anything slow.

Two things to watch:
- After an `await`, other requests may have run in between. Use `e.app`, not the global `$app`.
- A transaction holds SQLite's write lock, so keep it short across `await`s.

### Why there's no VM pool

goja runs JS synchronously: while a handler waits on the database or an HTTP call, its runtime can't do anything else.
To serve requests at the same time, PocketBase keeps a pool of runtimes (15 by default, `--hooksPool`) and makes more
when they're all busy. Each runtime loads every hook file separately. That's why a handler can't see the top of its own
file, why state can't be shared between requests, and why memory grows with load.

QuickJS here works like a Node.js process. Each app has one VM with an event loop. While a handler waits, the loop runs
other requests, and the handler picks up again when its result arrives. There's one copy of your code and your
globals, so nothing needs a pool (`--hooksPool` is accepted and ignored).

As in Node.js, the JS itself runs one piece at a time. A handler that computes for a long time holds the loop until it
finishes or hits its time budget. Keep heavy work out of the request path, or move it into Go.

## 6. Memory and time limits

Each app has a memory cap, and each task has a time budget:

```js
// pb_hooks/limits.pb.js
routerAdd("GET", "/hog", (e) => {
  const junk = []
  while (true) junk.push("x".repeat(1024 * 1024))
})

routerAdd("GET", "/spin", (e) => {
  while (true) {}
})
```

Both requests fail and the app keeps running:

| Request | Stopped after | Error in the request log |
|---|---|---|
| `/hog` | 0.08 s | `InternalError: out of memory at pb_hooks/limits.pb.js:3:36` |
| `/spin` | 5.0 s | `InternalError: interrupted at pb_hooks/limits.pb.js:5:27` |

Calls to `/hits` right after each one answered normally.

These are the limits in the `pocketone` binary:

| Limit | Default |
|---|---|
| Memory: the JS heap plus the Go objects JS holds | 64 MiB per app |
| JS time per task | 5 s |
| Wall time per task, Go calls included | 30 s |

Change them in `pocketone.yaml` ([pocketone.yaml](/docs/pocketone-yaml)):

```yaml
app:
  js_limits:
    memory: 128MiB # the JS heap plus the Go objects JS holds
    task_time: 10s # JS time per task
    task_wall: 60s # wall time per task, Go calls included
```

With `task_time: 1s`, `/spin` stopped after 1.0 s instead of 5. A custom Go build sets the same limits in
`jsvm.Config.VM` ([JSVM](/docs/extensions/jsvm#configure)). On a multisite host each app has its own VM and its own
limits, so one app's runaway script doesn't stop the others.

## How it performs

The same hook file, on the `pocketone` binary with each engine (QuickJS, and `--jsvm goja` with its default pool of 15):

```js
// pb_hooks/bench.pb.js
routerAdd("GET", "/b/ping", (e) => e.json(200, { ok: true }))
routerAdd("GET", "/b/db", (e) => e.json(200, { n: e.app.findRecordsByFilter("_superusers", "id != ''", "", 10, 0).length }))
routerAdd("GET", "/b/cpu", (e) => {
  const items = []
  for (let i = 0; i < 2000; i++) items.push({ id: i, name: "item" + i, tags: ["a", "b", "c"] })
  return e.json(200, { n: JSON.parse(JSON.stringify(items)).length })
})
routerAdd("GET", "/b/wait", (e) => { sleep(100); return e.json(200, { ok: true }) })
routerAdd("GET", "/b/waitasync", async (e) => { await sleep.async(100); return e.json(200, { ok: true }) })
```

Each route got 50 concurrent clients for 10 seconds (`hey -c 50 -z 10s`, after a 3-second warm-up), with
`--dev=false`, on an Apple M1 Max (10 cores):

| Route | QuickJS req/s | p50 | p99 | goja req/s | p50 | p99 |
|---|---|---|---|---|---|---|
| A JSON reply | 19,471 | 2.2 ms | 9.4 ms | 55,901 | 0.8 ms | 3.3 ms |
| A database query (10 rows) | 18,116 | 1.9 ms | 12.8 ms | 40,447 | 0.8 ms | 7.9 ms |
| CPU-heavy JS (2,000-object JSON round trip) | 84 | 591 ms | 648 ms | 409 | 91 ms | 525 ms |
| Waits 100 ms with `sleep(100)` | 19 | 1.93 s | 12.0 s | 491 | 102 ms | 113 ms |
| Waits 100 ms with `await sleep.async(100)` | 481 | 103 ms | 110 ms | – | – | – |

| Memory (resident) | QuickJS | goja |
|---|---|---|
| Idle, after start | 46 MB | 49 MB |
| After all the routes above | 313 MB | 3.1 GB |

What it shows:

- **goja does more requests a second.** Its pool runs handlers on every core at once. QuickJS runs an app's JS on one
  thread, like Node.js: about 18,000 short requests a second for one app, on one core.
- **QuickJS uses a tenth of the memory.** goja's runtimes each hold a copy of every hook file, and it makes more of them
  as load grows. On a host running many apps, that's the difference that counts.
- **Waiting costs nothing with `.async`.** `await sleep.async(100)` kept pace with goja's pool. A plain `sleep(100)`
  didn't, so use `.async` for anything slow.
- **CPU-heavy JS is slower.** Move heavy work into Go, a cron job, or a background task.

## Going back to goja

To run an app on PocketBase's own engine, set it in `pocketone.yaml`:

```yaml
app:
  jsvm: goja
```

You can also pass `--jsvm goja`. The startup lines list what that turns off: pages, `$mcp`, `$tokens`, `$bulkmail`, JS
contracts, shared globals and async handlers.

## Next

- [JSVM](/docs/extensions/jsvm): every file type, module resolution rule and setting.
- PocketBase's [JS docs](https://pocketbase.io/docs/js-overview/): the JS API itself, which is unchanged.
- [Teach your coding agent](/docs/agents): give your agent the PocketOne skill so it writes hooks the QuickJS way.
