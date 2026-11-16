---
owners: ["@benallfree"]
version: 0.0.1
---

# Mothership

A control plane for a fleet of edges. The mothership is a PocketOne app that records which edges exist and which
instances run where. Each edge follows it, serves its own instances and forwards requests for the rest to the edge
that runs them.

One operator, no users: owners and accounts are the [tenants](../tenants/README.md) extension (planned), which extends the
mothership through its collections and the mirror's extensions. Each layer is optional: an [edge](../edge/README.md)
runs without a mothership, and a mothership without tenants.

## Install

The edge side (the `mirror` package) is built into `pocketone`. It turns on when `pocketone.yaml` names a provider
and has a `mothership` section (below); `pocketone serve` then takes its instances and the other edges from the
mothership.

The mothership side has no switch yet. In the Go code of the PocketOne app that will be the mothership, create its
collections once the app has bootstrapped:

```go
if err := mothership.EnsureCollections(app); err != nil {
	return err
}
```

The module is `github.com/pockethost/pocketone/ext/mothership`. In your own edge build, `mirror.FromSection(log)`
returns the mirror for the config (nil without a `mothership` section); use it as the host's instance source and the
router's edge source, as [`examples/pocketone/multi.go`](../../examples/pocketone/multi.go) does.

## Configure

On each edge, add a `mothership` section to the host's config; the mirror becomes the host's [instance.source/v1](../contracts/instance/source/v1/README.md). Logins come from the private `.env`:

```yaml
multisite: {}
mothership:
  url: http://10.88.0.10:8090
  email: $MOTHERSHIP_EMAIL
  password: $MOTHERSHIP_PASSWORD
  self: lax-1                       # this edge's edge_id (or edges.self)
  internal_url: http://10.88.0.5:8080
  region: lax
  apex: example.com                 # <name>.example.com routes to each instance
```

`url` and `self` are required. The edge logs in as a superuser of the mothership.

On the mothership, there's nothing to configure beyond creating the collections.

## Use

You manage the fleet in the mothership's dashboard, in two collections only superusers can read or write:

- **`edges`**: `edge_id` (such as `lax-1`), `internal_url`, `region`, `status` (`online`, `draining`, `offline`) and
  `last_seen`. An edge with `internal_url` set registers itself when it has no record, marks itself online every
  15 s, and offline when it shuts down.
- **`instances`**: `name` (`<name>.<apex>` routes to it), `hosts` (extra hostnames), `edge` (where it runs),
  `power` (off answers 409 "instance is powered off"), `suspension` (non-empty answers 403 with that message), `dev`,
  `version`, `secrets`, `encryption_key`, `access`, `limits`, `plugins` (extension settings; the field keeps its old name) and `status`.

Changes reach the edges within moments over PocketBase's realtime stream; there's no polling. To move an instance,
change its `edge`: the old edge drops its full record and forwards to the new one until the new edge has it. An
instance with no `edge` isn't served anywhere.

## Integrations

### Works with

| Extension | How they fit |
|---|---|
| [Multisite](../multisite/README.md) | The mirror is the host's instance source: which instances exist and which this edge runs. |
| [Edge](../edge/README.md) | The mirror is also an edge source: where the other edges are, so requests for their instances are forwarded. It combines with a fixed `edges` list. |
| [Docker](../docker/README.md) and [Pocker](../pocker/README.md) | Run the instances this edge owns, with the version, secrets, limits and extension settings from each record. |
| [Tenants](../tenants/README.md) (planned) | Adds owners and accounts through mirror extensions. |

### Building on Mothership

- **Your own fields and collections.** `EnsureCollections` leaves fields an extension added alone, so an extension can add
  its own fields and collections to the mothership.
- **Mirror extensions.** Pass a `mirror.Extension` to `mirror.FromSection` or `mirror.Config.Extensions`. `Views()`
  adds filtered subscriptions (such as `owners` filtered to `instances_via_owner.edge ?= {edge}`), `Instance()`
  adjusts each instance as it's built (block a suspended owner's instances, add an owner's trusted IPs), and
  `Changed()` names the instances a change in its views affects.
- **Go API.** `mirror.New(mirror.Config{...})` builds a mirror without the config file. `Get(view, id)` and
  `Records(view)` read any view; `SelfRecord()` is this edge's record id.

## Example

There's no `example/main.go` yet. The stock binary's multi-instance mode,
[`examples/pocketone/multi.go`](../../examples/pocketone/multi.go), follows a mothership when the config has the
section.

## How it works

Each edge subscribes to filtered views, so it only receives what concerns it:

| View | Collection | Filter / fields | Why |
|---|---|---|---|
| `edges` | edges | all | forwarding, and health from heartbeats |
| `routes` | instances | fields `id,name,hosts,edge,updated` | the routing table: any edge may get any visitor |
| `local` | instances | `edge = <this edge>` | full records (secrets, limits, plugins) for instances this edge runs |

An instance moving away shows up in `routes` and its full record is dropped; one moving here arrives in `local`. The
stream reconnects and reloads on its own, and reloads everything every 10 minutes in case anything was missed.

Filters cut what each edge downloads and keeps (most edges run a few of thousands of instances). They are not a
security boundary while edges log in as superusers.

### Status

Early. Built: the `edges` and `instances` collections (`EnsureCollections`) and the edge side (`mirror`). Not yet:
the mothership's own endpoints (placement, moves) and instance status reports.

## License

MIT
