# ext: PocketOne extensions

Every extension is its own Go module (`github.com/pockethost/pocketone/ext/<name>`) with a README, so heavy
dependencies (Litestream, CertMagic) never reach core or other extensions. This folder is ours, separate from upstream's
`plugins/`, so upstream merges never touch it.

**Names.** An extension whose name starts with `multisite-` only works on a host that serves many instances
([`multisite`](multisite/README.md), with a provider such as
[`multisite-provider-pocker`](multisite-provider-pocker/README.md) or
[`multisite-provider-docker`](multisite-provider-docker/README.md)). Every other extension works on a single app,
and per instance under multisite.

Each extension's README has its docs, and is also its page on [pocketone.dev](https://pocketone.dev/docs/plugins).
How to write one: [docs/plugins.md](../docs/plugins.md).
