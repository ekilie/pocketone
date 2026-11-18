# Desktop changelog

Changes to `ext/desktop`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New extension: `pocketone desktop build` compiles the app in the current folder (`pb_hooks`, `pb_migrations`, `pb_public`) into a native desktop app that runs the app's server on a private port and shows it in a native window (Wails, the system webview). `--all` builds macOS, Windows and Linux (arm64 and amd64) from any machine with Go, no cgo; `--target` picks some. `pocketone.yaml`'s `desktop` section sets the app's name, id and window size. Each user's data lives in their app-data folder, named by the id.
