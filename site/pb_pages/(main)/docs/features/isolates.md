---
title: "PocketBase isolates: hundreds of apps in one process"
description: Run hundreds of PocketBase apps as isolates inside one pocketone process, routed by hostname. Each starts on its first request in tens of milliseconds, costs a few MB, and is sandboxed from the others.
---

# PocketBase isolates: hundreds of apps in one process

> **PocketBase:** one app per process; to host several, you run a process, container or VM for each. **PocketOne:** hundreds of apps as isolates inside one process. Each starts on its first request in tens of milliseconds, costs a few MB, and is sandboxed from the others.

An isolate is a complete PocketOne app: its own folder, `pb_data`, `pb_hooks`, `pb_migrations`, dashboard and
superusers, at its own hostname. What's different is where it runs. Instead of a process or container of its own, it
runs inside one shared `pocketone` process, with its own JavaScript VM, and the host routes each request to it by
hostname.

Most apps sit idle most of the time, so a process or container per app is mostly paying for nothing. Isolates make a
server full of PocketBase apps cheap: SaaS with one app per customer, a site per client, a preview app per branch.

It's built into `pocketone`. You don't need Docker, Kubernetes or a separate host program.

## Try it in a minute

Put this `pocketone.yaml` in an empty folder:

```yaml
server: { http: "127.0.0.1:8090" }
multisite: {}
instances: { dev: true }
pocker: { root: ./pb_root }
```

Start it, then ask for any hostname:

```bash
pocketone serve
curl -H 'Host: blog.localhost' http://127.0.0.1:8090/api/health
curl -H 'Host: shop.localhost' http://127.0.0.1:8090/api/health
```

In dev mode, each new hostname becomes an app: its folder appears under `pb_root/` and it starts on that first
request. Open `http://blog.localhost:8090/_/` for that app's own dashboard.

On a laptop, each new app answered its first request in about 45 ms, and later requests in under a millisecond. A
hundred running apps took the process from 53 MB to 365 MB: about 3 MB each.

## Run it for real

List your apps and their hostnames in an instances file:

```yaml
# instances.yaml
instances:
  - id: blog                          # also its folder: pb_root/blog
    hosts: [blog.example.com]
  - id: shop
    hosts: [shop.example.com]
    secrets: { STRIPE_KEY: sk_live_... }   # seen only by this app's hooks
    limits: { max_js_execution: 60s }
```

```yaml
# pocketone.yaml
multisite: {}
instances: { file: ./instances.yaml }
pocker: { root: ./pb_root }
```

The host checks the file every 2 seconds, so adding, changing or removing an app needs no restart. In production,
only the hostnames listed there are served. [Auto TLS](/docs/extensions/autotls) gets certificates for them.

## What each app gets

- **Everything a PocketOne app has.** [Point-in-time restore](/docs/features/restore), [API tokens](/docs/features/tokens),
  [MCP](/docs/features/mcp), [PocketPages](/docs/features/sites), [bulk mail](/docs/features/bulk-mail) and the rest
  are on in every isolate. Each can be turned off per app.
- **Its own deploys.** [`pocketone deploy`](/docs/features/deploy) and the [Remote CLI](/docs/features/remote) work
  against each app's hostname.
- **Its own restarts.** A change to its `pb_hooks`, a restore or its settings restarts that app alone. Nothing else
  notices.

## Sandboxed from each other

Isolates share a process, so PocketOne puts the walls inside it. Assume every app's owner is hostile:

- **Memory:** each app's JS has a hard cap (64 MiB by default). Going past it is an out-of-memory error for that app
  only.
- **CPU:** every JS call (hook, route, cron, migration) is stopped after `max_js_execution`. At most
  `max_concurrent_js` calls per app run at once.
- **Files, SQL and network:** an app's hooks and SQL reach only its own data. Outbound requests can't reach private
  networks unless you allow them.
- **Failures stay put:** a broken hooks file, a panic or a failed migration takes down that app alone. It answers 502
  until it's fixed.
- **Secrets:** each app's hooks see only its own `secrets`.

Every limit has a default for the whole host and an override per app. [Sandbox](/docs/extensions/sandbox) lists every
check.

## When to use containers instead

An isolate runs the PocketOne code in the `pocketone` binary. For an app that needs a specific PocketBase release, use
the [Docker](/docs/extensions/docker) runtime, which runs each app in its own container with the official binary of the
version it asks for. One host can run both.

## How it works

- [Multisite](/docs/extensions/multisite) takes over `serve` when `pocketone.yaml` has a `multisite` section. It
  routes each request by hostname, applies the gates, and hands it to a runtime.
- [Pocker](/docs/extensions/pocker) is the runtime that runs apps as isolates in the host's process.
- [Sandbox](/docs/extensions/sandbox) confines each isolate's JS, SQL and outbound network.
- [Docker](/docs/extensions/docker) is the container runtime, for apps that need it.
