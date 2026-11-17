---
owners: ["@benallfree"]
version: 0.0.1
title: "Vacuum"
group: binary
summary: "Compact your app's SQLite databases on a schedule, so space freed by deleted records goes back to the disk."
---

# Vacuum

SQLite doesn't give space back to the disk when records are deleted; the file stays as big as it ever was. Vacuum
compacts your app's databases on a schedule (SQLite's `VACUUM`), so the space comes back.

## Install

Vacuum is built into `pocketone`; a `vacuum` section in `pocketone.yaml` turns it on. In your own Go build:

```go
extkit.Setup(app, []extkit.Extension{vacuum.New()}, nil, nil)
```

## Configure

```yaml
vacuum:
  every: 168h   # weekly; any duration like 24h
```

Without the section, nothing runs.

## Use

There's nothing to click. Every `every`, while the app serves, Vacuum runs `VACUUM` on `data.db` and `auxiliary.db`
(PocketBase's own, online). While it runs, writes wait. The log says when it's done. `VACUUM` needs free disk space
about the size of the database it compacts.

## Integrations

| Extension | How they fit |
|---|---|
| [PITR](../pitr/README.md) | A compacted database is a new file to Litestream, so the next replication starts a fresh generation. |

### Building on Vacuum

`vacuum.App(app)` compacts a running app now, and `vacuum.Dir(folder)` compacts an app folder's databases while the
app isn't running.

## Example

A PocketBase app that compacts its databases once when it starts serving.

```go
// Command example is a PocketBase app that compacts its databases once when it starts serving (vacuum.App), the work
// the extension does on the `vacuum` section's schedule. Run `go run . serve` and watch the log for "vacuum: done".
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/vacuum"
)

func main() {
	app := pocketbase.New()
	if _, err := extkit.Setup(app, []extkit.Extension{vacuum.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		go func() {
			if err := vacuum.App(app); err != nil {
				app.Logger().Error("vacuum", "error", err)
			}
		}()
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## License

MIT
