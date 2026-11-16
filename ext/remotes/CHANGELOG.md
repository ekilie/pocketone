# Remotes changelog

Changes to `ext/remotes`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

## PocketOne v0.0.1-alpha.6 (2026-11-16)

### Upgrading

- `login` and `link` no longer take a password: `--email`, `--password`, `--collection` and `POCKETONE_PASSWORD` are gone. Sign in through the browser (`--device` on a machine without one), or pass an API token with `--token`, `--token-stdin` or `P1_TOKEN`. A saved login made with a password keeps working; `logout` no longer revokes its token, so delete it in the dashboard.
