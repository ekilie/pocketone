# Cache changelog

Changes to `ext/cache`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New standard extension: cache buckets for an app and its extensions (cache/v1) from one store per process. On a
  multisite host every instance shares it and sees only its own buckets; identical values are kept once. LRU, MRU,
  FIFO or LIFO eviction, a memory budget for the store and caps per bucket, TTLs, and an optional disk tier that
  survives restarts. Configure it in `pocketone.yaml`'s `cache` section.
