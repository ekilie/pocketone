---
title: Folders
description: What each pb_* folder next to your app holds, which ones are backed up, deployed or safe to delete, and where extensions keep their tables and files.
---

# Folders

A PocketOne app is a folder. PocketBase's own folders sit next to each other in it, and PocketOne adds `pb_cache`.

| Folder | What it holds | Backed up | Deployed and synced | Safe to delete |
|---|---|---|---|---|
| `pb_data` | the database, uploaded files, logs | yes | never | no: it's your data |
| `pb_hooks` | your JS hooks (and `*.pb.wasm` modules) | as code, in git | yes | no |
| `pb_migrations` | your app's migrations | as code, in git | yes | no |
| `pb_public` | static files the app serves | as code, in git | yes | no |
| `pb_pages` | PocketPages pages | as code, in git | yes | no |
| `pb_templates` | page templates you ejected (`pocketone template eject`) | as code, in git | yes | no |
| `pb_cache` | files extensions can rebuild, one folder each | never | never | yes, at any time |

Add `pb_data/` and `pb_cache/` to your `.gitignore`.

## `pb_cache`

Each extension that keeps disposable files has its own folder: `pb_cache/<extension>/` (say, `pb_cache/agent/` for
the built-in model's saved conversation state). Everything in it can be rebuilt, so deleting it is always safe: the
app carries on, perhaps a little slower until the cache fills again.

```bash
pocketone cache                # each extension's folder, its file count and size
pocketone cache clear          # empty every folder
pocketone cache clear agent    # empty one
```

On a multisite host every instance has its own `pb_cache` in its folder, and it counts against that instance's disk.

## Extension tables

Extensions that store records keep them in collections named `_<extension>_<table>` (`_bulkmail_campaigns`,
`_tokens_oauth_clients`), so you can tell them from your own collections at a glance and see which extension owns each.
They live in `pb_data` like the rest of your data, so they're backed up with it.

Extensions create and update those collections themselves when the app starts, with their own migrations. Those are
recorded apart from your `pb_migrations`, so `pocketone migrate` never touches them:

```bash
pocketone extensions migrations              # every extension's migrations and when each was applied
pocketone extensions migrate down bulkmail   # revert bulkmail's last migration (its data goes with it)
```
