Source: https://pocketone.dev/docs/features/restore

# Point-in-time restore

> **PocketBase:** backups you schedule, restored whole. **PocketOne:** every change is recorded, and you restore to any second in the last 72 hours.

Deleted the wrong records, ran a bad migration, or let a script loose on production? Pick a moment from before it
happened, and your app's data goes back to how it was then.

This is on for every PocketOne app. You don't need to set anything up.

## How far back you can go

PocketOne records every change to your database as it happens and keeps **the last 72 hours**. You can restore to any
second inside that window.

If history recording is turned off, the app restores from your PocketBase backups instead: the latest backup made at or
before the moment you pick. The steps below are the same either way. The app tells you which point it used.

## Restore from the dashboard

1. Sign in to the dashboard as a superuser and open **Settings → Backups**.
2. Find **Restore to a moment**. It shows how far back you can go.
3. Under **Restore to**, pick the date and time to go back to, then click **Restore**.

The app keeps serving while it rebuilds the old data, then restarts with it. That takes a few seconds for most apps.

## Changed your mind? Undo the restore

The data you replaced is kept, and changes keep being recorded after a restore. So after a restore, the section shows
when you restored and to what time, with an **Undo this restore** button. Click it to put things back as they were
just before you restored.

## What's covered

- **Restored:** your database: records, collections, rules, settings and superusers.
- **Not restored:** uploaded files, `pb_hooks`, `pb_public` and the logs. Keep those in your PocketBase backups, or in
  your own source control.

Only superusers can see or run a restore.

## From your terminal

Sign in to your app once ([Remote CLI](https://pocketone.dev/docs/features/remote)), then:

```bash
pocketone remote restore points        # how far back you can go
pocketone remote restore to --at -2h   # back to two hours ago
```

`--at` takes a duration from now (`-2h`, `-30m`), a local time (`2026-11-10 15:00`) or an RFC 3339 time
(`2026-11-10T15:00:00Z`).

On the machine that runs the app, you can also work from its files directly. Pull a copy of the old database to look
through without touching the live one, or restore in place while the server is stopped (it's swapped in at the next
start):

```bash
pocketone pitr restore --at 2026-11-10T15:00:00Z --to /tmp/then.db   # a copy, safe any time
pocketone pitr restore --at 2026-11-10T15:00:00Z                     # in place, server stopped
```

## From an agent

An agent connected to your app over [MCP](https://pocketone.dev/docs/reference/mcp) has two tools: `restore_points` to see how far back it
can go, and `restore_restore_to` to restore. It needs a token with the `restore` scope, and only a superuser can grant
one.

## Keep history somewhere safer

By default, history is kept on the same disk as your database. That protects you from mistakes, but not from losing
the disk. To keep it in your own S3-compatible bucket, open **Settings → Backups → Point-in-time restore**, tick **Store
the history in my own S3 bucket**, fill in the bucket details, click **Test connection**, then **Save**.

## Change how far back it goes, or turn it off

In `pocketone.yaml`:

```yaml
pitr:
  retention: 168h   # keep a week instead of 72 hours
  # enabled: false  # stop recording; restores then use your backups
```

A longer window uses more storage. Every key is in the [pocketone.yaml reference](https://pocketone.dev/docs/config-reference).

## How it works

- [PITR](https://pocketone.dev/docs/extensions/pitr) records the history with Litestream and restores to any second.
- [Backups](https://pocketone.dev/docs/extensions/backups) restores from PocketBase's own backups when PITR is off.
- Both answer the same [restore/v1](https://pocketone.dev/docs/contracts/restore/v1) contract, which is why the dashboard, the command and
  the MCP tools don't care which one is on.
