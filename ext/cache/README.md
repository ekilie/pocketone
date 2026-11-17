---
owners: ["@benallfree"]
version: 0.0.1
title: "Cache"
group: binary
summary: "Cache buckets for every app in the process from one shared store: identical values kept once, LRU/MRU/FIFO/LIFO eviction, TTLs, an optional disk tier."
---

# Cache

Extensions and apps often compute the same thing many times: a rendered page, an answer from another API, a compiled
module. Cache gives them named buckets to keep those results in, from one store in your app's process, with an
optional folder on disk for more than fits in memory. Each bucket has its own caps, and you can change them without
touching the code that uses them.

On a multisite host the store is shared by every instance: one memory budget and one disk tier for all of them. Each
instance still sees only its own buckets, and identical bytes (the same compiled template, the same `.wasm` file) are
kept once however many instances or keys hold them.

Cache provides [cache/v1](../contracts/cache/v1/README.md). Code that caches talks only to that contract, so a cache
shared between servers (Redis, Memcached) can replace this one later without any of it changing.

## Install

Cache is built into `pocketone` and on by default; there's nothing to install. Turn it off with
`cache: {enabled: false}` in `pocketone.yaml`. In your own Go build it comes with the standard set, or on its own:

```go
extkit.Setup(app, []extkit.Extension{cache.New(cache.Config{})}, nil, nil)
```

## Configure

Nothing to configure: the store has 128 MiB of memory for every bucket together, and each bucket takes the caps its
code asks for. To change them, add a `cache` section to `pocketone.yaml`. The top keys size the store; `defaults` fill
in what a bucket's code leaves unset; `buckets` set a named bucket's caps over what its code asks for:

```yaml
cache:
  max_memory: 512MiB       # the store: every bucket together
  disk_path: cache         # overflow for what memory evicts; leave out for memory only
  max_disk: 10GiB
  defaults:
    ttl: 10m
  buckets:
    pages.compiled:
      max_memory: 64MiB    # one app's bucket
```

The store:

| Key | What it does |
|---|---|
| `max_memory` | Memory for every bucket together (default 128MiB) |
| `max_items` | Most distinct values kept in memory; identical bytes count once |
| `eviction` | Which values leave memory first when the store is full: `lru` (default), `mru`, `fifo`, `lifo` |
| `disk_path` | Turns on the disk tier there, in a `p1cache` folder that is emptied when the process starts. Relative paths are under `pb_data` on a single app, and under the working directory on a multisite host |
| `max_disk` | Size of the disk tier (default 1GiB) |

Each bucket (`defaults` and `buckets.<name>`):

| Key | What it does |
|---|---|
| `max_items` | Most entries the bucket holds |
| `max_memory` | Most bytes of keys and values the bucket holds; a value it shares with others counts in full |
| `eviction` | Which of its entries go first when the bucket is full |
| `ttl` | How long an entry lives unless its code says otherwise |

Sizes take `KiB`, `MiB`, `GiB` (or `KB`, `MB`, `GB`); durations take `s`, `m`, `h`.

A disk tier inside `pb_data` is part of your backups (as throwaway files). To keep it out, give `disk_path` an
absolute folder outside `pb_data`.

## Use

Nothing to click. Values evicted from memory move to the disk tier when there is one, and a read brings them back.
The cache starts empty with the process, disk tier included. On a multisite host, an instance that restarts or
sleeps keeps its entries: the store belongs to the host.

## Integrations

| Extension | How they fit |
|---|---|
| [Pocker](../pocker/README.md) | Every instance shares the host's one store (its memory, its disk tier), and sees only its own buckets: keys live under the instance's id. An instance that sleeps keeps its entries, so it wakes to a warm cache. An instance's `extensions.cache` may set `enabled`, `defaults` and `buckets` for itself; the store's keys are the host's only. |
| [Docker](../docker/README.md) | Each container runs its own `pocketone`, so it has its own store. |

### Building on Cache

Open a bucket through [cache/v1](../contracts/cache/v1/README.md) at serve time, keep it, and get and set keys:

```go
b, err := cachev1.Open(app, "myext.answers", cachev1.Options{MaxItems: 1000, TTL: 10 * time.Minute})
```

Declare cache/v1 in `WorksWith`: when nothing provides it, `Open` returns a bucket that keeps nothing, so your code
runs uncached. Name buckets after your extension so they never collide.

To cache Go values rather than bytes, make them `cachev1.Item`s and open the bucket with `cachev1.OpenItems`
([Go values](../contracts/cache/v1/README.md#go-values)). Cache keeps each value as it is in memory and calls its
`MarshalCache` only to move it to the disk tier; a read from disk decodes it and keeps it in memory as a value again.

Cache provides cache/v1 as the default: another extension that provides it (a Redis cache, say) replaces it without
any `providers:` line.

## Example

A PocketBase app with one bucket, `example.slow`. `GET /slow` takes a second the first time and
answers from the cache for the next minute.

```go
// Command example is a PocketBase app with cache/v1 and one bucket, "example.slow". GET /slow takes a second the first
// time and answers from the cache for the next minute:
//
//	go run ./example serve --http=127.0.0.1:8090
//	curl http://127.0.0.1:8090/slow   # slow
//	curl http://127.0.0.1:8090/slow   # cached
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password, app user user@example.com / password.
package main

import (
	"log"
	"net/http"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"github.com/pockethost/pocketone/ext/cache"
	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/extkit"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"
	if _, err := extkit.Setup(app, []extkit.Extension{cache.New(cache.Config{})}, nil, nil); err != nil {
		log.Fatal(err)
	}
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		b, err := cachev1.Open(app, "example.slow", cachev1.Options{MaxItems: 100, TTL: time.Minute})
		if err != nil {
			return err
		}
		se.Router.GET("/slow", func(e *core.RequestEvent) error {
			ctx := e.Request.Context()
			if v, ok, _ := b.Get(ctx, "answer"); ok {
				return e.String(http.StatusOK, "cached: "+string(v)+"\n")
			}
			time.Sleep(time.Second) // the expensive part
			v := time.Now().Format(time.RFC3339)
			_ = b.Set(ctx, "answer", []byte(v), 0)
			return e.String(http.StatusOK, "computed: "+v+"\n")
		})
		return se.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## How it works

One store per process. Each app's buckets are namespaces in it, `<instance id>/<bucket>` (just `/<bucket>` on a
single app), so no app can read another's keys. Keys point at values stored by their SHA-256: the same bytes under
any number of keys, from any number of instances, are one value with a reference count, and count once against the
store's memory and item limits. A value goes when the last key pointing at it does.

There are two kinds of eviction. When the store is over its limits, a value leaves memory (by the store's strategy):
to the disk tier when there is one, else together with every key pointing at it. When one bucket is over its own
caps, one of its keys goes (by the bucket's strategy), and the value only if nothing else points at it.

Go values (`cachev1.Item`) have no bytes to hash, so each is its own value under a random name, never shared, and
counts its `CacheSize` against the limits. It's marshaled only when it spills to the disk tier, by the IO goroutine,
which records the file's hash for later reads to check.

The key index stays in memory and counts against `max_memory`. The disk tier is overflow for memory: one file per
value, named by its hash, written atomically and emptied when the process starts.

One lock guards the index, and it's never held across disk IO. One IO goroutine, running only while it has work,
writes and removes files in the order the lock decided them. A value waiting to be written stays readable from
memory (at most a quarter of `max_memory` waits; past that, evicted values are dropped). Reads from disk happen off
the lock, many readers of one value share one read, and a read is checked against the index again before it's
returned, so a key changed meanwhile never gets its old value back.

The store is tested against a model under fuzzing (no value deleted, replaced, expired or belonging to another key
ever comes back; every count and list stays consistent; the disk tier holds exactly the files it should) and under
concurrent load with the race detector.

## License

MIT
