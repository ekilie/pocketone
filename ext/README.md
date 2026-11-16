# ext: PocketOne extensions

Every extension is its own Go module (`github.com/pockethost/pocketone/ext/<name>`) with a README, so heavy
dependencies (Litestream, CertMagic) never reach core or other extensions. This folder is ours, separate from upstream's
`plugins/`, so upstream merges never touch it.

Most are **app extensions**: they run on every app, a standalone one or each instance of a multisite host. Every app
has the [standard set](standard/README.md). A few are **host extensions** for a [multisite](multisite/README.md) host,
the process that serves many instances: firewall, autotls, servertiming, edge, mothership. The host runs its instances
with a runtime provider, [pocker](pocker/README.md) (in-process) or [docker](docker/README.md) (containers).

Each extension's README has its docs, and is also its page on [pocketone.dev](https://pocketone.dev/docs/extensions).
How to write one: [docs/extensions.md](../docs/extensions.md).
