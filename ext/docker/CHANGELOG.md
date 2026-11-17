# Docker changelog

Changes to `ext/docker`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

## 0.0.2 (2026-11-17)

### Fixed

- A container that exits right after it starts no longer loses what it printed: the runtime attaches to its output before starting it, so the reason it exited reaches `OnLog` (and `remote/v1`'s logs) even though Docker removes it at once.

## PocketOne v0.0.1-alpha.3 (2026-11-16)

### Added

- A host with its own accounts decides who may call `remote/v1` (status, restart, logs) for its containers with `docker.Provider.RemoteAuth`; without it, the instance's own superuser login is checked as before. `remote/v1` now lives on the host's contract scope, so `pocketone remote whoami` also lists the host's other contracts for the instance (a host's `deploy/v1`).
