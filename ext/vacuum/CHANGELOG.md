# Vacuum changelog

Changes to `ext/vacuum`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New extension: a `vacuum` section (`every: 168h`) compacts the app's SQLite databases (`data.db`, `auxiliary.db`) on a schedule, online.
