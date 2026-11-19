---
owners: ["@benallfree"]
version: 0.0.1
title: "Settings"
group: binary
summary: "One API for the settings an app's admins change for its extensions. On by default."
---

# Settings

One API for the settings an app's admins change for its extensions: PITR's S3 bucket, Bulk Mail's sending settings,
and any extension that registers a section. The dashboard screens, the CLI and agents all use it.

## Install

Settings is built into `pocketone` and **on by default**; there's nothing to install. In your own Go build:

```go
extkit.Setup(app, []extkit.Extension{settings.New(), pitr.New(pitr.Config{})}, nil, nil)
```

The module is `pocketone.dev/ext/settings`.

## Configure

There's nothing to configure.

## Use

It provides the [settings/v1](../contracts/settings/v1/README.md) contract (superusers only):

```bash
curl -H "Authorization: $TOKEN" https://app.example.com/api/_c/settings/v1                      # which extensions
curl -H "Authorization: $TOKEN" "https://app.example.com/api/_c/settings/v1/get?extension=pitr" # one extension's
```

Agents get the same through MCP (`settings_list`, `settings_get`, `settings_save`, `settings_test`).

## Integrations

| Extension | How they fit |
|---|---|
| [PITR](../pitr/README.md) | Its S3 destination is a section. |
| [Bulk Mail](../bulkmail/README.md) | Its sending settings are a section. |
| [Contracts](../contracts/README.md) | settings/v1 is the contract; a host could provide it instead. |

## Example

```go
// Command example is a PocketBase app with settings/v1: the standard extensions' admin settings (here pitr's) behind
// one API. `curl -H "Authorization: <superuser token>" http://127.0.0.1:8090/api/_c/settings/v1` lists them.
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password.
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/pitr"
	"pocketone.dev/ext/settings"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"
	if _, err := extkit.Setup(app, []extkit.Extension{settings.New(), pitr.New(pitr.Config{})}, nil, nil); err != nil {
		log.Fatal(err)
	}
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## License

MIT
