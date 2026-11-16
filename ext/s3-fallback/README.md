---
owners: ["@benallfree"]
version: 0.0.1
---

# S3 Fallback

A default bucket for instances that have no S3 of their own. You name your S3-compatible backends once, then mount
each kind of file on one: record uploads, backup zips, PITR history, or any extension's own kind. Every instance gets
its own folder in the bucket and can't reach anything outside it. The credentials never reach the instance.

## Install

S3 Fallback isn't built into `pocketone`; add it to a Go build. Under [Pocker](../pocker/README.md), add it to the
server's extensions:

```go
srv := &pocker.Server{Config: cfg, Extensions: []pocker.Extension{s3fallback.New(s3fallback.Config{})}}
```

On a single app, register it with the backends and mounts in Go:

```go
extkit.MustRegister(app, s3fallback.New(s3fallback.Config{Backends: backends, Mounts: mounts}))
```

The module is `github.com/pockethost/pocketone/ext/s3-fallback`.

## Configure

Under Pocker the `storage` section of `pocketone.yaml` is read at start. Fields set in Go win.

```yaml
storage:
  backends:                        # named S3-compatible destinations
    r2-eu:   { endpoint: https://<account>.eu.r2.cloudflarestorage.com, bucket: pocketbase-eu, region: auto, access_key: $R2_KEY, secret: $R2_SECRET }
    b2-cold: { endpoint: ..., bucket: ph-backups, region: ..., access_key: ..., secret: ... }
  mounts:                          # filesystem kind -> backend
    storage:    r2-eu              # record uploads
    backups:    b2-cold            # backup zips
    litestream: r2-eu              # PITR history (ext/pitr)
  # enabled: false                 # off for every instance
```

Each backend also takes `prefix` (prepended to every key, so several deployments can share a bucket) and
`force_path_style` (for MinIO and similar servers). With no mounts the extension does nothing and files stay on local
disk, as in PocketBase.

Each instance can turn it off in its own settings, from `instances.yaml` or a mothership. Only `enabled` is read per
instance; backends and mounts are the operator's:

```yaml
instances:
  - id: blog
    extensions: { storage: { enabled: false } }
```

The old shape (`bucket:` with `uploads:` / `backups:`) still works, with a warning: it becomes a backend named
`default`, mounted for uploads and backups.

## Use

There's nothing to do day to day. To an instance nothing is configured: Settings show S3 off, and uploads, thumbs,
`$app.newFilesystem()` and the Backups screen work as before. Files land in the bucket under
`<prefix>instances/<id>/<kind>/`. Downloads stream through the server, or redirect to the bucket with
[S3 Redirect](../s3redirect/README.md).

When an instance turns on its own S3 (Settings → Files storage, or Settings → Backups), PocketBase's normal code takes
over with the instance's credentials. Files already in the default bucket stay there, as PocketBase's own S3 switch
doesn't move files either.

It applies to instances created with the extension on. Files already on an instance's local disk are not read or moved.

## Integrations

### Works with

| Extension | How they fit |
|---|---|
| [Pocker](../pocker/README.md) | Each instance gets the root `instances/<id>/`; the `storage` section is read when the server starts. |
| [PITR](../pitr/README.md) | Mount `litestream` and every instance without its own bucket keeps its history there. An instance's own PITR bucket still wins. |
| [S3 Redirect](../s3redirect/README.md) | Downloads become signed redirects to the bucket. Nothing to wire: the fenced filesystem signs with its prefix, so S3 Redirect treats it like any S3 mount. |

### Building on S3 Fallback

- **Your own kind.** Any kind an extension asks for through PocketBase's `OnFilesystemCreate` is served when it's mounted.
  PITR's `litestream` works this way. Uploads and backups step aside when the app turned on its own S3; for other
  kinds, the extension asking checks its own app setting first.
- **Go API.** `Bind(app, root)` serves an app's filesystems from the mounts under `root` (`""` for a single app,
  `instances/<id>/` under Pocker). `FS(root, kind)` returns the same fenced filesystem outside a hook; close it when
  done. `Config()` returns the effective configuration.

## Example

There's no `example/main.go` yet. [`storage_test.go`](storage_test.go) and [`mounts_test.go`](mounts_test.go) show
it on a single app and under Pocker, against MinIO.

## How it works

- PocketBase asks `OnFilesystemCreate` before it creates a filesystem. For a mounted kind, the extension answers with the
  backend, fenced to `<backend prefix><root><kind>/`. An instance can't list, read, write or delete anything outside
  it.
- The fence is a driver that adds the prefix to every key and strips it from listings. Signed download URLs are made
  with the backend's own credentials.

### Status

Works and is tested against MinIO; not yet deployed.

## License

MIT
