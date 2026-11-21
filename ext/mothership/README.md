---
owners: ["@benallfree"]
version: 0.0.1
title: "Mothership"
group: many
summary: "A control plane and its edges, across regions."
---

# Mothership

A control plane for a fleet of edges. The mothership is a PocketOne app that records which edges exist and which
instances run where. Each edge asks it about a hostname the first time a request names one, follows changes to that
hostname on one stream, serves its own instances and forwards requests for the rest to the edge that runs them.

One operator, no users: owners and accounts are the [tenants](../tenants/README.md) extension (planned), which adds
its collections and wraps [instance.resolve/v1](../contracts/instance/resolve/v1/README.md). Each layer is optional:
an [edge](../edge/README.md) runs without a mothership, and a mothership without tenants.

## Install

Both sides are built into `pocketone`; `pocketone.yaml`'s `mothership` section picks one.

- **The mothership** is a single app with `serve: true`. It keeps the `edges` and `instances` collections (`Schema`)
  and serves [instance.watch/v1](../contracts/instance/watch/v1/README.md) to the edges (`Server`).
- **An edge** is a multisite host with `url` and `self`. It takes its instances from the mothership
  ([instance.source/v1](../contracts/instance/source/v1/README.md) and
  [instance.lookup/v1](../contracts/instance/lookup/v1/README.md)) and the other edges from it.

The module is `pocketone.dev/ext/mothership`. In your own build: `extkit` registers `mothership.NewSchema()` and
`mothership.NewServer()` on the mothership app (a control plane with its own collections registers only the server,
beside its own instance.resolve/v1); on an edge, `client.FromSection(log)` returns the client for the config (nil
without a `url`), a multisite host extension, as [`examples/pocketone/binary/multi.go`](../../examples/pocketone/binary/multi.go)
does. The client keeps its answers in the host's cache/v1, so the host provides one (`cache.Extension.Host(scope)`).

## Configure

On the mothership:

```yaml
mothership:
  serve: true
  apex: example.com                 # <name>.example.com routes to each instance
```

On each edge, with logins from the private `.env`:

```yaml
multisite: {}
mothership:
  url: http://10.88.0.10:8090
  email: $MOTHERSHIP_EMAIL
  password: $MOTHERSHIP_PASSWORD
  self: lax-1                       # this edge's edge_id (or edges.self)
  internal_url: http://10.88.0.5:8080
  region: lax
  max_hosts: 50000                  # hostnames the edge keeps answers for (cache bucket mothership.hosts)
```

`url` and `self` are required on an edge. The edge logs in as a superuser of the mothership.

## Use

You manage the fleet in the mothership's dashboard, in two collections only superusers can read or write:

- **`edges`**: `edge_id` (such as `lax-1`), `internal_url`, `region`, `status` (`online`, `draining`, `offline`) and
  `last_seen`. An edge with `internal_url` set registers itself when it has no record, and marks itself online every
  15 s.
- **`instances`**: `name` (`<name>.<apex>` routes to it), `hosts` (extra hostnames), `edge` (where it runs),
  `power` (off answers 409 "instance is powered off"), `suspension` (non-empty answers 403 with that message), `dev`,
  `version`, `secrets`, `encryption_key`, `access`, `limits`, `plugins` (extension settings; the field keeps its old name) and `status`.

A change reaches every edge that has served the hostname within moments, on its stream; there's no polling and no full
copy. To move an instance, change its `edge`. An instance with no `edge` isn't served anywhere; a hostname no instance
has answers 404, and the edge hears when one takes it.

## Integrations

### Works with

| Extension | How they fit |
|---|---|
| [Multisite](../multisite/README.md) | The client is the host's instance source and lookup: which instance a hostname serves, asked on first use. |
| [Edge](../edge/README.md) | The client is also an edge source: where the other edges are, so requests for their instances are forwarded. It combines with a fixed `edges` list. |
| [Cache](../cache/README.md) | The client keeps answers in the host's bucket `mothership.hosts` (required). |
| [Docker](../docker/README.md) and [Pocker](../pocker/README.md) | Run the instances this edge owns, with the version, secrets, limits and extension settings from each record. |
| [Tenants](../tenants/README.md) (planned) | Adds owners and accounts by wrapping instance.resolve/v1. |

### Building on Mothership

- **Your own fields and collections.** `EnsureCollections` leaves fields an extension added alone.
- **Your own answers.** Wrap instance.resolve/v1 (`resolvev1.Contract.Wrap`) to adjust what a hostname serves (block a
  suspended owner's instances, add an owner's trusted IPs), or provide your own over your own collections. List the
  records an answer read in its deps, and report from `Touches` the names and keys a write changes.
- **The standing set.** `Standing(edge)` lists instances an edge must hold without traffic (work addressed to it); they
  arrive in the stream's hello and whenever the set changes.
- **Go API.** `client.New(client.Config{...})` builds an edge client without the config file; `Standing()`,
  `EdgeRecords()` and `SelfRecord()` read what the stream sent.

## Example

There's no `example/main.go` yet. The stock binary is both sides: [`examples/pocketone/binary/binary.go`](../../examples/pocketone/binary/binary.go)
registers the mothership, and its multi-instance mode, [`examples/pocketone/binary/multi.go`](../../examples/pocketone/binary/multi.go),
follows one; `examples/pocketone/mothership_test.go` runs a mothership and two edges.

## How it works

- **One stream per edge** (`GET /api/_c/instance.watch/v1/stream?edge=<id>`). Its hello carries the connection id,
  every edge and the edge's standing set.
- **Lookup on a miss.** A hostname the edge's cache doesn't hold is watched on the stream; the mothership registers the
  watch, then resolves, and sends the answer on the stream. Requests for one hostname share one ask.
- **Changes.** After every committed write, the mothership resolves again each watch whose deps the record touches,
  and sends only answers that changed. A cached hostname is updated; one the cache dropped is unwatched.
- **Stream down.** The edge keeps serving what it cached; a miss is answered in the reply, used once and not cached.
- **Reconnect.** A new stream is a new generation: the cache key carries it, so everything from before is ignored
  (and cleared). A mothership restart drops every stream, and every edge starts over.

Edges log in as superusers today, so the edge names itself on the stream; a per-edge credential will carry it instead.

### Status

Early. Built: the collections, the watch server, the default resolver and the edge client. Not yet: the mothership's
own endpoints (placement, moves), instance status reports, per-edge credentials.

## License

MIT
