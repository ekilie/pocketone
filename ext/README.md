# ext: PocketOne extensions

Every extension is its own Go module with a README, so heavy dependencies (Litestream, CertMagic) never reach core or
other extensions. This folder is ours, separate from upstream's `plugins/`, so upstream merges never touch it.

Extensions live in one of two folders:

- **[`core/`](core)**: shipped and maintained by PocketOne. Module path `github.com/pockethost/pocketone/ext/core/<name>`.
- **`community/`**: contributed extensions, each owned by whoever submitted it. Module path
  `github.com/pockethost/pocketone/ext/community/<name>`.

**Names.** An extension whose name starts with `multisite-` only works on a host that serves many instances
([`multisite`](core/multisite/README.md), with a provider such as
[`multisite-provider-pocker`](core/multisite-provider-pocker/README.md) or
[`multisite-provider-docker`](core/multisite-provider-docker/README.md)). Every other extension works on a single app,
and per instance under multisite.

Each extension's README has its docs, and is also its page on [pocketone.dev](https://pocketone.dev/docs/plugins).
How to write one: [docs/plugins.md](../docs/plugins.md).
