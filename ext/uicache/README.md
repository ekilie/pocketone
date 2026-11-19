---
owners: ["@benallfree"]
version: 0.0.2
title: "UI Cache"
group: binary
summary: "Dashboard cache headers that let updates through browsers and CDNs. On by default."
---

# UI Cache

PocketBase tells browsers and CDNs to keep every dashboard file for 14 days, including files whose name never changes.
Behind a CDN such as Cloudflare, that means a new or updated dashboard screen from an extension, or the dashboard's own
libraries after an upgrade, can take up to two weeks to show up. UI Cache sends headers that let those changes through
within minutes, and keeps the long cache for the files that can safely have it.

## Install

UI Cache is built into `pocketone` and **on by default**; there's nothing to install. In your own Go build:

```go
extkit.Setup(app, []extkit.Extension{uicache.New()}, nil, nil)
```

The module is `pocketone.dev/ext/uicache`.

## Configure

There's nothing to configure. An app turns it off with `uicache: {enabled: false}` in its pocketone.yaml, which brings
back PocketBase's own headers.

## Use

Nothing to do day to day. Check the headers with curl:

```bash
curl -sI https://app.example.com/_/extensions.js | grep -i cache-control
```

| Dashboard path | Cache-Control | Why |
|---|---|---|
| `/_/extensions.js`, `/_/index.html` | `no-cache` | Checked on every load: `extensions.js` is every extension's screen joined into one file, rebuilt when any of them changes. |
| `/_/assets/<name>-<hash>.js` (and `.css`) | PocketBase's own (14 days) | The name changes with the content, so a new build is a new URL. |
| Everything else (`libs/`, `fonts/`, `images/`, `/_/extensions/<name>/...`) | `public, max-age=300` | Five minutes: fresh soon after an update, still cached while you browse. |

`/_/` itself (the dashboard page) is sent without a cache lifetime by PocketBase and left alone. In dev mode
(`--dev`) UI Cache does nothing, as PocketBase sends no cache headers there.

After turning it on for an app that sits behind a CDN, purge `/_/*` there once: copies cached under the old 14-day
header stay until they expire.

## Integrations

| Extension | How they fit |
|---|---|
| [Multisite](../multisite/README.md) | Every instance gets it with the standard set; turn it off for one with `extensions.uicache.enabled: false`. |
| Extensions with dashboard screens ([Tokens](../api-tokens/README.md), [PITR](../pitr/README.md), [Bulk Mail](../bulkmail/README.md), ...) | Their `main.js` reaches the browser through `/_/extensions.js`, so an updated screen shows on the next load. |

An extension that serves other files from `/_/extensions/<name>/` gets the five-minute policy. To cache one longer,
put a content hash in its name and set its own `Cache-Control` in a middleware that runs before UI Cache's
(`uicache.MiddlewareID`); a header that's already set is kept.

## Example

A PocketBase app with UI Cache, to see the headers locally.

```go
// Command example is a PocketBase app whose dashboard sends cache headers that let changes through: run
// `go run . serve`, then `curl -sI http://127.0.0.1:8090/_/extensions.js | grep -i cache-control` shows `no-cache`.
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/uicache"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"
	if _, err := extkit.Setup(app, []extkit.Extension{uicache.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## How it works

PocketBase's dashboard middleware (in `apis/serve.go` and `apis/extensions.go`) sets its 14-day `Cache-Control` only
when the response has none yet. UI Cache binds a router middleware in `OnServe`, which runs before route and group
middleware, and sets the header first. No PocketBase code changes.

## License

MIT
