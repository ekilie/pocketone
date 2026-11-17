# Config changelog

Changes to `ext/config`, newest first. A user-visible change adds its line under `## Unreleased

### Upgrading

1. Move `public_dir`, `index_fallback`, `hooks_watch` and `automigrate` from the `app` section to the `server` section of `pocketone.yaml`. An old key under `app` now stops startup as unknown.

### Changed

- `app` holds what every command that opens the app reads (`dir`, `dev`, `encryption_env`, `query_timeout`, `jsvm`, `hooks_dir`, `hooks_pool`, `migrations_dir`, `env_allow`); `server` holds what `serve` reads, now including `public_dir`, `index_fallback`, `hooks_watch` and `automigrate`.

### Added

- `server.pages_dir`: the PocketPages folder, the `--pagesDir` default.` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased
