# S3 Fallback changelog

Changes to `ext/s3-fallback`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Changed

- S3 Fallback is a standard extension: built into `pocketone` and on for every app and isolate, doing nothing until the `storage` section mounts a backend. A Go build that added it with `set.Add(s3fallback.New(...))` sets `set.S3Fallback` instead.
