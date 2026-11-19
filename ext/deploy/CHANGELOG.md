# Deploy changelog

Changes to `ext/deploy`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

## 0.1.0 (2026-11-18)

### Upgrading

1. Deploy history is renamed from `_deploys` to `_deploy_deploys` (its id, `pbc_deploys`, stays) on the first start, by the extension's migration. Code that reads it by name switches to `_deploy_deploys`, or to the id.

### Changed

- Deploy history is the `_deploy_deploys` collection, following the `_<extension>_<table>` rule; an extension migration renames it.
- `deploy`, `dev` and the deploy set never include `pb_cache` (extensions' disposable files), like `pb_data`.

## 0.0.2 (2026-11-17)

### Changed

- `pocketone pull` also fetches `pb_templates` by default (ejected page templates, ext/templates). `deploy` and `dev` already sync it with the other `pb_*` folders.

## PocketOne v0.0.1-alpha.2 (2026-11-16)

### Changed

- Two-way `pocketone dev` sync, and a deploy never restarts the app on its own (`deploy --restart`, `POST /restart`, or `r` in dev).
