# Cache changelog

Changes to `ext/cache`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New standard extension: cache buckets for an app and its extensions (cache/v1) from one store per process. On a
  multisite host every instance shares it and sees only its own buckets. Each bucket has its own budget (entries,
  memory, disk) with LRU, MRU, FIFO or LIFO eviction and TTLs, under the store's memory and disk ceilings. An optional
  disk tier holds what memory evicts, keeping identical bytes once. Configure it in `pocketone.yaml`'s `cache` section.
- Go values in the cache: each key keeps its own `cachev1.Item` in memory; a `cachev1.Marshaler` is marshaled only to
  go to disk, and read back only by a decoder of its `CacheVersion` (`cachev1.OpenItems`, `cachev1.ItemBucket`).
- Emptying the cache: `cachev1.Clear` (every bucket of the app), cache.clear/v1 for the app's admins
  (`pocketone remote cache clear [--bucket <name>]`), and the extension's `Clear` for a host (the whole store).
