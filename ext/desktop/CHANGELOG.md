# Desktop changelog

Changes to `ext/desktop`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Changed

- The build image is now `ghcr.io/pocketonehq/pocketone-build:v<version>` (the repo moved to the `pocketonehq` org); earlier versions keep pulling their image from `ghcr.io/pockethost/pocketone-build`.


## 0.0.2 (2026-11-18)

### Added

- New extension: `pocketone desktop build` compiles the app in the current folder (`pb_hooks`, `pb_migrations`, `pb_public`) into a native desktop app that runs the app's server on a private port and shows it in a native window (Wails, the system webview). `--all` builds macOS, Windows and Linux (arm64 and amd64), no cgo; `--target` picks some. Builds run in Docker, in the build image for your `pocketone`'s version (Go and PocketOne's source, vendored; works offline once pulled); `--local` builds with your own Go and a checkout instead. `pocketone.yaml`'s `desktop` section sets the app's name, id and window size. Each user's data lives in their app-data folder, named by the id. On a Mac, each macOS build also comes as a `.dmg`. A `sign` section in `desktop` signs every build on your machine (`--no-sign` skips it): macOS with your Developer ID, notarized and stapled; Windows with your signing command; Linux with a `.sha256` and a GPG `.asc`; tools and keys are checked before the build starts.

