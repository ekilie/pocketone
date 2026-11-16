# Deploy changelog

Changes to `ext/deploy`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

## PocketOne v0.0.1-alpha.2 (2026-11-16)

### Changed

- Two-way `pocketone dev` sync, and a deploy never restarts the app on its own (`deploy --restart`, `POST /restart`, or `r` in dev).
