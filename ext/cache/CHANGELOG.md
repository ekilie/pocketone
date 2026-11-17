# Cache changelog

Changes to `ext/cache`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New extension: cache buckets for an app and its extensions (cache/v1), in memory with LRU, MRU, FIFO or LIFO
  eviction, item and memory caps, TTLs and an optional disk tier that survives restarts. Override any bucket's limits
  in `pocketone.yaml`'s `cache` section.
