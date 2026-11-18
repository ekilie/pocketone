---
title: pocketone.yaml
description: Put your app's settings in one file instead of a line of flags, keep secrets in a .env beside it, and set any extension's options.
---

# pocketone.yaml

> **PocketBase:** app settings live in the dashboard, and `serve` takes its options as command-line flags. **PocketOne:** one file next to your app holds them, every extension's options and a `.env` for secrets.

Every `pocketone` flag has a default. `pocketone.yaml` lets you change those defaults in one file, next to your app,
instead of typing them on every start. The same file holds each extension's options, and your secrets go in a `.env`
beside it.

Without the file, nothing changes: PocketOne behaves like stock PocketBase. Every step below was run on `pocketone`.

## 1. Write the file

In your app's folder, create `pocketone.yaml`:

```yaml
server:
  http: 127.0.0.1:8090
app:
  dir: ./pb_data
  dev: false
```

Check it, create a superuser, and start the app:

```bash
pocketone config check
# pocketone.yaml: ok (2 sections)
pocketone superuser upsert admin@example.com password
pocketone serve
```

`pocketone serve` listens on the address under `server`, and the data folder is `pb_data` next to the file. Relative
paths are relative to the file's folder, so the app starts the same from any working directory.

PocketOne looks for `pocketone.yaml` in the working directory, then next to the executable. `--config <path>` picks
another file.

## 2. Flags still win

A flag always beats the file. The file only changes the defaults:

```bash
pocketone serve --http=127.0.0.1:8091   # listens on 8091, whatever the file says
```

## 3. Keep secrets in a .env

Put secrets in a `.env` beside the file, and refer to them as `$VAR` or `${VAR}` in any value:

```bash
# .env
LISTEN=127.0.0.1:8090
STRIPE_KEY=sk_test_123
OTHER_SECRET=hidden
```

```yaml
server:
  http: ${LISTEN}
```

Your JS hooks don't see the `.env` unless you allow each name:

```yaml
app:
  env_allow: [STRIPE_KEY]
```

```js
// pb_hooks/env.pb.js
routerAdd("GET", "/env", (e) => {
  return e.json(200, { stripe: process.env.STRIPE_KEY || null, other: process.env.OTHER_SECRET || null })
})
```

```bash
curl http://127.0.0.1:8090/env
# {"stripe":"sk_test_123","other":null}
```

`pocketone config check` says when it read secrets from a `.env`. Keep the `.env` out of git.

## 4. Set an extension's options

Each top-level key is a section owned by one extension. For example, keep three days of point-in-time history, and
give the app's JS more room:

```yaml
pitr:
  retention: 72h
app:
  js_limits:
    memory: 128MiB
    task_time: 10s
```

`js_limits` sets the limits of the JS engine your hooks run on; [QuickJS hooks](/docs/quickjs#6-memory-and-time-limits)
shows what they stop.

To see every section your binary knows, with each key, run:

```bash
pocketone config
```

The [pocketone.yaml reference](/docs/config-reference) lists the same, with each key's type, default and meaning.

## 5. Choose which extension does a job

Extensions offer their abilities through [contracts](/docs/how-pocketone-fits-together). When more than one
extension can do the same job, `providers` picks one. Both PITR and Backups can restore your data. PITR is used when
it's on, but you can choose Backups:

```yaml
providers:
  restore/v1: backups
```

The startup lines show the choice:

```
INFO standard: restore/v1 ← backups (providers:)
```

## The whole file

```yaml
server:
  http: ${LISTEN}
app:
  dir: ./pb_data
  dev: false
  env_allow: [STRIPE_KEY]
  js_limits:
    memory: 128MiB
    task_time: 10s
pitr:
  retention: 72h
providers:
  restore/v1: backups
```

## Next

- [pocketone.yaml reference](/docs/config-reference): every section and key.
- [Config](/docs/extensions/config): how the file is found, `.env` rules, and loading it in your own Go build.
