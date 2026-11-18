# Standard Set changelog

Changes to `ext/standard`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New member `WASM` (ext/wasm): WASI modules in `pb_hooks`, called with `$wasm`. Reconfigure it with `set.WASM = wasm.New(wasm.Config{...})`. Turn it off on a single app with `wasm: {enabled: false}`.

### Fixed

- A single app (the stock `pocketone` binary) now honors `enabled: false` in pocketone.yaml for every standard member: `settings`, `templates`, `tokens`, `remotes`, `mcp`, `pages`, `backups`, `bulkmail` and `deploy` each get a section with just `enabled` (before, those sections were refused as unknown and the switch was never read). Extensions that require the one you turn off go off too, and the startup lines say why.

## 0.0.2 (2026-11-17)

### Added

- New member `Cache` (ext/cache): cache buckets for extensions and apps (cache/v1), in memory with an optional disk tier. Turn it off per app with `cache: {enabled: false}`.
- New member `Templates` (ext/templates): extensions' page templates, ejected to `pb_templates` to edit (`pocketone template eject oauth` for the sign-in screens). Turn it off per app with `templates: {enabled: false}`.

