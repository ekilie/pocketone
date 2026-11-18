# Standard Set changelog

Changes to `ext/standard`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New member `WASM` (ext/wasm): WASI modules in `pb_hooks`, called with `$wasm`. Reconfigure it with `set.WASM = wasm.New(wasm.Config{...})`.

## 0.0.2 (2026-11-17)

### Added

- New member `Cache` (ext/cache): cache buckets for extensions and apps (cache/v1), in memory with an optional disk tier. Turn it off per app with `cache: {enabled: false}`.
- New member `Templates` (ext/templates): extensions' page templates, ejected to `pb_templates` to edit (`pocketone template eject oauth` for the sign-in screens). Turn it off per app with `templates: {enabled: false}`.

