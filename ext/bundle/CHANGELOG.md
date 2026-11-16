# Bundle changelog

Changes to `ext/bundle`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

## PocketOne v0.0.1-alpha.6 (2026-11-16)

### Upgrading

- `ext/jsembed` is now `ext/bundle` and runs on the `ext/jsvm` engine. Import `github.com/pockethost/pocketone/ext/bundle`, register `ext/jsvm` instead of `plugins/jsvm`, and pass `bundle.Hooks(...)` / `bundle.Migrations(...)` in `jsvm.Config.Extensions`. A `require()` that isn't found now says "Cannot find module".

### Added

- `bundle.Public(app, fsys, indexFallback)` serves public files embedded in the binary, like `pb_public`.
