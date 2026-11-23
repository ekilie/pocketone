---
title: Migrations
description: Migrations carry your collection changes and seed data with your code - automigrate writes them when you change collections in the dashboard, deploys apply them, and you can write your own in TypeScript or Go.
---

# Migrations

Your app's schema (its collections, fields, rules and indexes) lives in its database, but it belongs with your code: the
copy on your laptop and the one on your server need the same collections. Migrations are how: small files in
`pb_migrations/`, each with an **up** (make the change) and a **down** (undo it), that every copy of the app runs once,
in order.

## Automigrate: change the dashboard, get a file

With automigrate on (the default for `pocketone serve`), every collection change you save in the dashboard writes a
migration file:

```
pb_migrations/
  1732300000_created_posts.js
  1732300512_updated_posts.js
```

Commit those files with your code. When the app starts somewhere else, or when you run
[`pocketone deploy`](/docs/features/deploy), it applies the ones it hasn't run yet. So the usual flow is: change
collections in your local dashboard, then deploy.

Turn it off with `--automigrate=false` or `automigrate: false` in [pocketone.yaml](/docs/pocketone-yaml)'s `server`
section, for example on a production server where nobody should change collections by hand.

## Write one yourself

For seed data, data fixes, or changes you'd rather write than click:

```bash
pocketone migrate create seed_categories
# pb_migrations/1732400000_seed_categories.js
```

Rename it `.ts` if you like; PocketOne runs TypeScript migrations directly.

```ts
// pb_migrations/1732400000_seed_categories.ts
migrate((app) => {
  const categories = app.findCollectionByNameOrId("categories")
  for (const name of ["News", "Guides", "Releases"]) {
    const record = new Record(categories)
    record.set("name", name)
    app.save(record)
  }
}, (app) => {
  for (const name of ["News", "Guides", "Releases"]) {
    const record = app.findFirstRecordByData("categories", "name", name)
    app.delete(record)
  }
})
```

```go
// migrations/1732400000_seed_categories.go
package migrations

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		categories, err := app.FindCollectionByNameOrId("categories")
		if err != nil {
			return err
		}
		for _, name := range []string{"News", "Guides", "Releases"} {
			record := core.NewRecord(categories)
			record.Set("name", name)
			if err := app.Save(record); err != nil {
				return err
			}
		}
		return nil
	}, func(app core.App) error {
		for _, name := range []string{"News", "Guides", "Releases"} {
			record, err := app.FindFirstRecordByData("categories", "name", name)
			if err != nil {
				return err
			}
			if err := app.Delete(record); err != nil {
				return err
			}
		}
		return nil
	})
}
```

- The `app` a migration gets runs inside a transaction: if the up throws, nothing it did is kept.
- Files run in the order of their names, which start with a Unix time, so new files run after old ones.
- A Go build imports its migrations package (`_ "yourapp/migrations"`) so its files register; they run with the JS
  ones, in one order.
- [Collections and fields](/docs/collections) shows creating and changing collections in a migration.

## Run, undo and inspect

```bash
pocketone migrate up          # apply every migration not yet applied (serve does this at start)
pocketone migrate down 1      # undo the last one
pocketone migrate collections # write one file with every collection as it is now (a snapshot)
pocketone migrate history-sync # forget applied migrations whose files are gone
```

The app records applied migrations in its `_migrations` table, so each runs once per database.

A snapshot (`migrate collections`) is a good way to start over: one file that creates every collection as it is, in
place of a long history. Delete the old files, keep the snapshot, and run `history-sync` on each existing copy.

## Extensions bring their own

PocketOne's extensions keep their own tables (the LLM ledger, API tokens, bulk mail) with their own migrations, tracked
apart from yours. They never appear in `pb_migrations`, and you don't need to copy their schema.

## Next

- [Collections and fields](/docs/collections): what migrations change.
- [Deploy and live sync](/docs/features/deploy): how migrations reach your server.
- [CLI reference](/docs/reference/cli): every `migrate` option.
