# PocketOne changelog

PocketOne's own changes, newest first: core, the `pocketone` binary, the site, and anything that isn't one
extension's. Each extension keeps its own changelog in its folder (`ext/<name>/CHANGELOG.md`); a release here lists
every extension that got a new version, with a link to its changes. Upstream PocketBase's changes are in
[CHANGELOG.pocketbase.md](CHANGELOG.pocketbase.md); each release here says which PocketBase version it's built on.

## How this file is kept

- **Every commit to `main` with a user-visible change adds its line under `## Unreleased` in the same commit:** a
  feature, a fix, a changed behavior or default, a removal, or a new upstream PocketBase version. Docs-only and
  internal changes don't need one. **A change to one extension goes in that extension's `CHANGELOG.md` instead**, under
  its own `## Unreleased`, with the same headings ([Changelogs](docs/extensions.md#changelogs)).
- **Group lines under these headings, in this order, and leave out empty ones:**
  - **Upgrading:** what someone on the previous release must do, as steps. Required for anything breaking.
  - **Added**
  - **Changed**
  - **Fixed**
  - **Extensions:** written by the release step, never by hand: each extension's new version, linked to its changes.
  - **PocketBase:** the upstream version this is built on, when it changes.
- **Write for someone running the binary:** what changed and what to do, with the command, flag or config key.
- **One line per change.** GitHub shows every newline as a line break, so don't hard-wrap lines.
- **Cutting a release:**
  1. Run `go run ./.github/extensions release vX.Y.Z`. It raises the version of every extension with lines under its
     `## Unreleased` (minor when it has upgrade steps, else patch) and stamps that section with it, renames this
     file's `## Unreleased` to `## vX.Y.Z (YYYY-MM-DD)` with an Extensions list, and starts new empty `## Unreleased`
     sections. Review and commit.
  2. Run `examples/pocketone/release-notes.sh vX.Y.Z > /tmp/notes.md`.
  3. Run `goreleaser release --config examples/pocketone/.goreleaser.yaml --release-notes /tmp/notes.md`.

  The release header adds the install and upgrade steps on top. Release notes are this file's section for the version,
  with its links pointing at the release on GitHub.

## Unreleased

### Upgrading

1. Put app and serve flags after the command name: `pocketone serve --hooksDir x`, not `pocketone --hooksDir x serve`. Only `--config` and `--env-file` (and core's `--dir`, `--dev`, `--encryptionEnv`, `--queryTimeout`, still accepted anywhere) can come before it.
2. In `pocketone.yaml`, move `public_dir`, `index_fallback`, `hooks_watch` and `automigrate` from `app:` to `server:` (see the config changelog).

### Added

- Agent Skills: a third skill, `pocketone-extensions`, for writing an extension and contributing it as a pull request; the `pocketone` skill now covers only using PocketOne in an app, and no longer carries its own copy of the PocketPages docs (the `pocketpages` skill has them). Update with `/plugin marketplace update pocketone` or `npx skills add pockethost/pocketone`.

### Changed

- Each flag is now on the commands that read it instead of on every command: `--dir`, `--dev`, `--encryptionEnv`, `--queryTimeout`, `--jsvm`, `--hooksDir`, `--hooksPool` and `--migrationsDir` on the commands that open the app (`serve`, `migrate`, `superuser`, `pitr`, `token`, `oauth`, `template`); `--hooksWatch`, `--automigrate`, `--publicDir`, `--indexFallback`, `--pagesDir`, `--pitr*`, `--s3Redirect*` and `--autotls*` on `serve` only. Remote commands such as `deploy` and `link` list just their own flags plus `--config` and `--env-file`, and reject the others.

## v0.0.1-alpha.6 (2026-11-16)

### Upgrading

- Environment variables: every PocketOne variable now starts with `P1_`, in every build (`PB_` stays PocketBase's own). Rename `POCKETONE_TOKEN` → `P1_TOKEN`, `POCKETONE_TOKEN_<REMOTE>` → `P1_TOKEN_<REMOTE>`, `POCKETONE_CONFIG_DIR` → `P1_CONFIG_DIR` and `PB_MAILTRAP_HTTP`/`PB_MAILTRAP_SMTP` → `P1_MAILTRAP_HTTP`/`P1_MAILTRAP_SMTP` in CI and scripts. The `pockethost` build reads `P1_TOKEN` too, no longer `POCKETHOST_TOKEN`.

### Added

### Changed

- A request with an unknown or expired `Authorization` token now gets a 401 instead of going on as a guest (PocketBase v0.42.0). Clients that send a stale token should drop it and sign in again. `/mcp` keeps answering with its own sign-in challenge, so MCP clients still re-authenticate.
- **contracts:** contract providers and `on<Contract><Op>` handlers written in pb_hooks can use the file's top-level variables.

### Fixed

- A crash or data race when an app stops its cron scheduler (on shutdown or restart), a PocketBase v0.42 regression; fixed in core until upstream has it.

### PocketBase

- Built on PocketBase v0.42.2 (was v0.41.1). See its [changelog](CHANGELOG.pocketbase.md) for v0.42.0 to v0.42.2.

## v0.0.1-alpha.4 (2026-11-16)

### Upgrading

### Changed

## v0.0.1-alpha.3 (2026-11-16)

### Upgrading

### Added

- `pocketone --version` prints the PocketBase version it's built on: `0.0.2 (PocketBase 0.41.1)`.

### Changed

- The startup lines listing the standard extensions and contracts print when the app serves, not before every command (`--version`, `help`, `migrate`).

### Fixed

## v0.0.1-alpha.2 (2026-11-16)

Built on PocketBase v0.41.1.

### Upgrading

- If you installed the old `v0.0.1`, reinstall (`brew reinstall pockethost/tap/pocketone`, or download the zip): `v0.0.1` became `v0.0.1-alpha.1`, and later alphas won't replace it on their own.

### Added

- **Extension contracts:** every extension API is a versioned contract, so extensions depend on what they need, not on each other by name.
- **Config:** `pocketone config --json` lists every section, key, type and default. The `pitr`, `autotls` and `s3redirect` sections set a single app's defaults; flags still win.
- **Dashboard:** the PocketOne logo, favicon and version in the superuser dashboard.
- Every extension's docs are on [pocketone.dev](https://pocketone.dev), now with search.

### Changed

- **`pocketone upgrade`** keeps every old executable as `pocketone-<version>` and follows prereleases: from one alpha to the next it just upgrades; from a stable release to a prerelease it asks first.
- **Config file is `pocketone.yaml`.**
- **Hooks:** in dev mode a broken `pb_hooks` file is skipped instead of stopping the app; a bad reload keeps the last good code.

### Fixed

- The hooks watcher's goroutine is stopped before bootstrap is cleared (a race in PocketBase's notify watcher).

## v0.0.1-alpha.1 (2026-11-16)

The first alpha, first published as `v0.0.1`. Built on PocketBase v0.41.1.

### Added

- Runs your existing `pb_data`, `pb_hooks` and `pb_migrations`.
- **pages** (formerly PocketPages): server-rendered pages from `pb_pages`; start with `pocketone pages create myapp`.
- **mcp** at `/mcp`, **Bulk Mail**, **Auto TLS**, **API tokens**, an **OAuth server**, **S3 redirects**, and `pocketone deploy | dev | pull | logs`.
- A dev-mode mail trap (`serve --dev`) at http://127.0.0.1:8025.
