---
owners: ["@benallfree"]
version: 0.0.1
title: "Cache"
group: services
summary: "Cache buckets in the app's own memory, with LRU/MRU/FIFO/LIFO eviction, TTLs and an optional disk tier."
---

# Cache

Extensions and apps often compute the same thing many times: a rendered page, an answer from another API, a resized
image. Cache gives them named buckets to keep those results in, in your app's own memory, with an optional folder on
disk for more than fits in memory. Each bucket has its own limits, and you can change them without touching the code
that uses them.

Cache provides [cache/v1](../contracts/cache/v1/README.md). Code that caches talks only to that contract, so a cache
shared between servers (Redis, Memcached) can replace this one later without any of it changing.

## Install

In your own Go build, add it to your extensions:

```go
extkit.Setup(app, []extkit.Extension{cache.New(cache.Config{})}, nil, nil)
```

or to the standard set: `set.Add(cache.New(cache.Config{}))`.

## Configure

Nothing to configure: each bucket uses the limits its code asks for, and 32 MiB of memory unless it asks for other.
To change them, add a `cache` section to `pocketone.yaml`. `defaults` fill in what a bucket's code leaves unset;
`buckets` set a named bucket's limits over what its code asks for:

```yaml
cache:
  defaults:
    max_memory: 16MiB
    ttl: 10m
  buckets:
    pages.render:
      eviction: lru       # lru, mru, fifo or lifo
      max_items: 5000
      max_memory: 128MiB
      disk_path: cache    # under pb_data; leave out for memory only
      max_disk: 1GiB
```

| Key | What it does |
|---|---|
| `eviction` | What a full bucket drops first: `lru` (least recently used, the default), `mru` (most recently used), `fifo` (oldest written), `lifo` (newest written) |
| `max_items` | Most entries kept in memory |
| `max_memory` | Most bytes kept in memory (default 32MiB per bucket) |
| `ttl` | How long an entry lives unless its code says otherwise |
| `disk_path` | Turns on the disk tier in this folder (relative paths are under `pb_data`) |
| `max_disk` | Most bytes the disk tier holds (default 256MiB per bucket) |

Sizes take `KiB`, `MiB`, `GiB` (or `KB`, `MB`, `GB`); durations take `s`, `m`, `h`.

A disk tier inside `pb_data` is part of your backups. To keep it out, give `disk_path` an absolute folder outside
`pb_data`.

## Use

Nothing to click. Entries evicted from memory move to the disk tier when there is one, and a read brings them back.
What's on disk is still there after a restart; what's in memory isn't. To empty a disk tier, stop the app and delete
its folder.

## Integrations

| Extension | How they fit |
|---|---|
| [Pocker](../pocker/README.md) | Every instance has its own buckets and its own limits. An instance's `extensions.cache` takes the same keys as the `cache` section and wins over it. An absolute `disk_path` gets the instance's id added, so instances never share files. |

### Building on Cache

Open a bucket through [cache/v1](../contracts/cache/v1/README.md) at serve time, keep it, and get and set keys:

```go
b, err := cachev1.Open(app, "myext.answers", cachev1.Options{MaxItems: 1000, TTL: 10 * time.Minute})
```

Declare cache/v1 in `WorksWith`: when nothing provides it, `Open` returns a bucket that keeps nothing, so your code
runs uncached. Name buckets after your extension so they never collide.

Cache provides cache/v1 as the default: another extension that provides it (a Redis cache, say) replaces it without
any `providers:` line.

## Example

A PocketBase app with one bucket, `example.slow`, with a disk tier. `GET /slow` takes a second the first time and
answers from the cache for the next minute.

```go
// Command example is a PocketBase app with cache/v1 and one bucket, "example.slow", with a disk tier. GET /slow takes a
// second the first time and answers from the cache for the next minute:
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
		b, err := cachev1.Open(app, "example.slow", cachev1.Options{MaxItems: 100, TTL: time.Minute, DiskPath: "cache"})
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

Each bucket is a map plus a list in use order, under one lock. A full bucket evicts by its strategy before it adds
an entry; an entry larger than the whole memory tier goes straight to disk, or isn't kept. Expired entries are
dropped when read, and in a sweep every few writes. The disk tier is one file per entry (named by the SHA-256 of its
key, with its key and expiry in the file), written atomically, and indexed when the bucket opens, oldest file first.
Disk reads and writes happen under the bucket's lock, so a delete never races a write to disk.

## License

MIT
