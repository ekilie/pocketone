# Upgrade changelog

Changes to `ext/upgrade`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

## 0.0.3 (2026-11-21)

### Changed

- `pocketone upgrade` on an executable installed with Scoop tells you to run `scoop update pocketone` instead, as it does for Homebrew.
- `pocketone upgrade` on an executable installed from a Linux package (in `/usr/bin`) tells you to run the install script instead, which installs the newer package.

## 0.0.2 (2026-11-18)

### Changed

- Releases come from `pocketonehq/pocketone` by default (the repo moved there from `pockethost/pocketone`, which redirects).

