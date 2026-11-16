# PocketOne changelog

PocketOne's own changes, newest first. Upstream PocketBase's changes are in [CHANGELOG.md](../CHANGELOG.md) at the
repo root; each release here says which PocketBase version it's built on.

## How this file is kept

- **Every commit to `main` with a user-visible change adds its line under `## Unreleased` in the same commit:** a
  feature, a fix, a changed behavior or default, a removal, or a new upstream PocketBase version. Docs-only and
  internal changes don't need one.
- **Group lines under these headings, in this order, and leave out empty ones:**
  - **Upgrading:** what someone on the previous release must do, as steps. Required for anything breaking.
  - **Added**
  - **Changed**
  - **Fixed**
  - **PocketBase:** the upstream version this is built on, when it changes.
- **Write for someone running the binary:** what changed and what to do, with the command, flag or config key. Start
  extension-specific lines with the extension's name, for example "**pages:**".
- **One line per change.** GitHub shows every newline as a line break, so don't hard-wrap lines.
- **Cutting a release:**
  1. Rename `## Unreleased` to `## vX.Y.Z (YYYY-MM-DD)`, put a new empty `## Unreleased` above it, and commit.
  2. Run `examples/pocketone/release-notes.sh vX.Y.Z > /tmp/notes.md`.
  3. Run `goreleaser release --config examples/pocketone/.goreleaser.yaml --release-notes /tmp/notes.md`.

  The release header adds the install and upgrade steps on top. Release notes are this section and nothing else.

## Unreleased

## v0.0.1-alpha.2 (2026-11-16)

Built on PocketBase v0.41.1.

### Upgrading

- If you installed the old `v0.0.1`, reinstall (`brew reinstall pockethost/tap/pocketone`, or download the zip): `v0.0.1` became `v0.0.1-alpha.1`, and later alphas won't replace it on their own.

### Added

- **Extension contracts:** every extension API is a versioned contract, so extensions depend on what they need, not on each other by name.
- **Config:** `pocketone config --json` lists every section, key, type and default. The `pitr`, `autotls` and `s3redirect` sections set a single app's defaults; flags still win.
- **tokens:** custom scopes for apps and extensions (Go guards, `$tokens` in JS, scope headers for collection rules), and any auth record can own tokens. OAuth sign-in goes through PocketBase's own auth endpoints.
- **Dashboard:** the PocketOne logo, favicon and version in the superuser dashboard.
- Every extension's docs are on [pocketone.dev](https://pocketone.dev), now with search.

### Changed

- **`pocketone upgrade`** keeps every old executable as `pocketone-<version>` and follows prereleases: from one alpha to the next it just upgrades; from a stable release to a prerelease it asks first.
- **Config file is `pocketone.yaml`.**
- **mcp:** every tool is one REST call run as the caller; connections stay signed in; the consent page can limit an agent to some scopes.
- **deploy:** two-way `pocketone dev` sync, and a deploy never restarts the app on its own (`deploy --restart`, `POST /restart`, or `r` in dev).
- **Hooks:** in dev mode a broken `pb_hooks` file is skipped instead of stopping the app; a bad reload keeps the last good code.

### Fixed

- The hooks watcher's goroutine is stopped before bootstrap is cleared (a race in PocketBase's notify watcher).

## v0.0.1-alpha.1 (2026-11-16)

The first alpha, first published as `v0.0.1`. Built on PocketBase v0.41.1.

### Added

- Runs your existing `pb_data`, `pb_hooks` and `pb_migrations`.
- **pitr:** point-in-time restore (Litestream), from Settings → Backups or `pocketone pitr restore`.
- **pages** (formerly PocketPages): server-rendered pages from `pb_pages`; start with `pocketone pages create myapp`.
- **mcp** at `/mcp`, **Bulk Mail**, **Auto TLS**, **API tokens**, an **OAuth server**, **S3 redirects**, and `pocketone deploy | dev | pull | logs`.
- A dev-mode mail trap (`serve --dev`) at http://127.0.0.1:8025.
