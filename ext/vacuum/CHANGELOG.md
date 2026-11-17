# Vacuum changelog

Changes to `ext/vacuum`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New extension: compacts the app's SQLite databases (`data.db`, `auxiliary.db`) on a schedule, only when enough of a database is free space. The app's admins turn it on and set how often in the Vacuum section of Settings → Backups, which also shows the last and next run; `pocketone.yaml`'s `vacuum` section sets the defaults. Its status is vacuum/v1. On a host that provides vacuum/v1 for its instances, it steps aside and the section shows the host's status, read-only.
