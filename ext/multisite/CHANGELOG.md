# Multisite changelog

Changes to `ext/multisite`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

## 0.0.2 (2026-11-17)

### Changed

- A multisite host refuses `server.public_dir`, `index_fallback`, `pages_dir`, `hooks_watch` and `automigrate`, the single-app settings now in that section.

