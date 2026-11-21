# Mothership changelog

Changes to `ext/mothership`, newest first. A user-visible change adds its line under `## Unreleased

### Upgrading

1. Edges follow the mothership lazily over instance.watch/v1 instead of PocketBase's realtime: run the mothership with `mothership: { serve: true, apex: <your apex> }` (the edges' `apex` moves there) before upgrading the edges. Go: the `mirror` package is gone; use `client.FromSection` / `client.New`, and provide cache/v1 on the host (`cache.Extension.Host`).

### Added

- instance.watch/v1 (`Server`): each edge opens one stream, asks for a hostname the first time a request names it, and hears its changes after that; the mothership resolves through instance.resolve/v1 (`Schema` provides the default over its collections).
- The edge client (`client`): instance.source/v1 and instance.lookup/v1 from the mothership, answers kept in the host's cache bucket `mothership.hosts`, served from cache while the mothership can't be reached.
` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased
