### Works with

| Plugin | How they fit |
|---|---|
| [Pocker](../../pocker/README.md) | Every instance on the host gets PITR. The `pitr` section sets the defaults for all of them; each instance can override `enabled` and `retention` (below). A restore restarts only that instance, and sleeping instances cost nothing. |
| [S3 Fallback](../../storage/README.md) | Mount `litestream` on a backend and every app without its own bucket keeps its history there, under its own prefix. An app's own bucket (Settings → Backups) still wins. |
| [Multitenant](../../tenants/README.md) (planned) | Sets each instance's `enabled` and `retention` from its plan, for example a plan without PITR. |
| PocketBase backups | `pb_data/litestream/` is left out of backup zips, and restoring a zip doesn't wipe it. |

### Per-instance settings (pocker)

Each instance can override `enabled` and `retention` in its `plugins.pitr` settings, from `instances.yaml` or a
provider:

```yaml
instances:
  - id: abc
    plugins: { pitr: { retention: 168h } }
  - id: free-plan-app
    plugins: { pitr: { enabled: false } }
```

Each instance's history goes to `instances/<id>/litestream/` in the shared bucket, or its own `pb_data/litestream/`.
A bucket in an instance's settings is refused, so credentials never sit in instance data.

### Building on PITR

- **Store history somewhere else.** PITR asks PocketBase for a filesystem of kind `litestream` (`pitr.Kind`) through
  `OnFilesystemCreate`. Answer it and the history goes to your filesystem; S3 Fallback works this way.
- **Set it per instance.** Write `plugins.pitr.enabled` and `plugins.pitr.retention` into an instance's settings;
  PITR reads them when the instance starts. Multitenant will work this way.
- **Go API.** `(*pitr.Plugin).Status(ctx, app)` returns the restorable window, size, destination and last restore;
  `Restore(ctx, app, at)` restores an app to a time.
- **REST API** (superusers): `GET /api/pitr` for the status, `POST /api/pitr/restore` with
  `{"at": "<RFC 3339 time>"}` to restore.
