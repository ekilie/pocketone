# Edge changelog

Changes to `ext/edge`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Fixed

- A request held for an owner edge that restarted goes through once the owner answers its health check. Each check used to time out after the retry interval (250 ms), shorter than a fresh connection to a far edge takes, so an entry edge far from a restarted owner held every request for it until it was restarted itself.
