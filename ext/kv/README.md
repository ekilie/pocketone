---
owners: ["@benallfree"]
version: 0.0.1
title: "Key-Value Store"
group: binary
summary: "Named JSON values for an app's settings and state, with no collection or migration. On by default."
---

# Key-Value Store

A place for an app's settings and small pieces of state: a name and a JSON value, kept without a collection or a
migration. Values live in PocketBase's own `_params` table, so they ride along in backups. Page code reads and writes
them with `$kv`, Go code with the [kv/v1](../contracts/kv/v1/README.md) helpers, and superusers through the API, the
CLI or an agent.

## Install

The Key-Value Store is built into `pocketone` and **on by default**; there's nothing to install. In your own Go build:

```go
extkit.Setup(app, []extkit.Extension{kv.New()}, nil, nil)
```

The module is `pocketone.dev/ext/kv`.

## Configure

There's nothing to configure. To turn it off on a single app (`$kv` and the API go away; the helpers keep working):

```yaml
kv:
  enabled: false
```

## Use

In page code and `pb_hooks`, `$kv` is sync and local:

```js
const defaults = { title: "Story Time", minutes: 10 }
const settings = $kv.get("storytime/settings") ?? defaults // the stored value, or null when there is none
$kv.set("storytime/settings", { ...settings, minutes: 15 })  // any value JSON.stringify takes, up to 1 MiB
$kv.delete("storytime/settings")                             // no error when there was no value
$kv.list("storytime/")                                       // [{ name, value, created, updated }], sorted by name
```

Names are 1 to 128 characters: letters, digits and `_ - . : /`, starting with a letter or digit. Start yours with a
prefix of your own (`storytime/`) so they stay apart from other code's.

Superusers (or an API token with the `kv` / `kv:read` scope) reach the same values over HTTP:

```bash
curl -H "Authorization: $TOKEN" "https://app.example.com/api/_c/kv/v1?prefix=storytime/"             # list
curl -H "Authorization: $TOKEN" "https://app.example.com/api/_c/kv/v1/get?name=storytime/settings"   # one value
curl -H "Authorization: $TOKEN" -d '{"name":"storytime/settings","value":{"minutes":15}}' \
  https://app.example.com/api/_c/kv/v1/set
curl -H "Authorization: $TOKEN" -d '{"name":"storytime/settings"}' https://app.example.com/api/_c/kv/v1/delete
```

The same operations are `pocketone remote kv ...` commands and MCP tools (`kv_list`, `kv_get`, `kv_set`, `kv_delete`).

## Integrations

| Extension | How they fit |
|---|---|
| [Contracts](../contracts/README.md) | kv/v1 is the contract. Other extensions keep their own values with its Go helpers (`kvv1.Get(app, name, &v)`, `kvv1.Set`, `kvv1.Delete`, `kvv1.List`), which need no provider, and declare `WorksWith: kv/v1`. |
| [Pages](../pages/README.md) | Page code gets `$kv`. |
| [API Tokens](../api-tokens/README.md) | Tokens with the `kv:read` or `kv` scope reach the API. |
| [MCP](../mcp/README.md) | Agents get the operations as tools. |
| [Multisite](../multisite/README.md) | Every instance that runs PocketOne has its own store, in its own `_params`. |

## Example

The example serves a PocketBase app with the store, and keeps a greeting in it from Go:

```go
// Command example is a PocketBase app with the key-value store (kv/v1): page code and pb_hooks get $kv, and
// superusers read and change values at /api/_c/kv/v1, e.g.
// `curl -H "Authorization: <superuser token>" "http://127.0.0.1:8090/api/_c/kv/v1/get?name=example/greeting"`.
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	kvv1 "pocketone.dev/ext/contracts/kv/v1"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/kv"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"
	if _, err := extkit.Setup(app, []extkit.Extension{kv.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	// a value from Go, kept until someone changes it
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		var greeting string
		if found, err := kvv1.Get(e.App, "example/greeting", &greeting); err != nil {
			return err
		} else if !found {
			if err := kvv1.Set(e.App, "example/greeting", "Hello from the key-value store"); err != nil {
				return err
			}
		}
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## License

MIT
