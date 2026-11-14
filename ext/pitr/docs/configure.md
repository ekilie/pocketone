### Single app

| Flag | Default | What it does |
|---|---|---|
| `--pitr` | `true` | Record history while serving. `--pitr=false` turns it off. |
| `--pitrRetention` | `72h` | How far back a restore can go. |

### Where the history is stored

By default the history is kept next to your database, in `pb_data/litestream/`. That protects you from mistakes in
your data, but not from losing the disk. To keep it somewhere else, use your own S3-compatible bucket:

1. Open **Settings → Backups** in the dashboard and find **Point-in-time restore**.
2. Tick **Store the history in my own S3 bucket** and fill in the endpoint, bucket, region, access key and secret.
   Tick **Force path-style addressing** for MinIO and similar servers. The prefix defaults to `litestream`.
3. Click **Test connection** to check that the bucket accepts a write, then **Save**.

From then on the history is recorded in the bucket, starting with a fresh copy, so restores go back to the moment you
saved. The secret is never shown again after saving. If the app runs with an encryption key (`--encryptionEnv`), the secret
is stored encrypted.

On a host that gives every instance shared storage ([S3 Fallback](/docs/plugins/storage)), history goes there unless
the app has its own bucket.

### Many instances (pocker)

The `pitr` section of `pocketone.yaml` sets the defaults for every instance; each instance can override `enabled` and
`retention` in its own settings (for example, off for a plan without PITR):

```yaml
pitr:
  retention: 72h
  snapshot_interval: 24h

# instances.yaml
instances:
  - id: abc
    plugins: { pitr: { retention: 168h } }
```
