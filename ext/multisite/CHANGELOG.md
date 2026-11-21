# Multisite changelog

Changes to `ext/multisite`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Upgrading

1. Go: `Host.Runtimes` and `Handler.Runtimes` are now `Runtime` (one runtime/v1 provider); set `Runtime: rt` instead of `Runtimes: []multisite.Runtime{rt}`.

### Changed

- A host runs one runtime: runtime/v1 has one provider (two registered conflict at startup unless `providers: { runtime/v1: <name> }` picks), and an instance the runtime can't serve gets 503. Instances no longer name a runtime.

## 0.0.2 (2026-11-17)

### Changed

- A multisite host refuses `server.public_dir`, `index_fallback`, `pages_dir`, `hooks_watch` and `automigrate`, the single-app settings now in that section.

