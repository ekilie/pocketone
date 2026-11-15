# ext: PocketOne extensions

Every extension is its own Go module (`github.com/pockethost/pocketone/ext/<name>`) with a README, so heavy
dependencies (Litestream, CertMagic) never reach core or other extensions. This folder is ours, separate from upstream's
`plugins/`, so upstream merges never touch it.

Some extensions only work on a host that serves many instances
([`multisite`](multisite/README.md), with a provider such as
[`pocker`](pocker/README.md) or
[`docker`](docker/README.md)). Every other extension works on a single app,
and per instance under multisite.

Each extension's README has its docs, and is also its page on [pocketone.dev](https://pocketone.dev/docs/extensions).
How to write one: [docs/extensions.md](../docs/extensions.md).
