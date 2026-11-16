# Templates changelog

Changes to `ext/templates`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New standard extension: `pocketone template list` shows the template sets extensions offer and what you've changed; `pocketone template eject [set]` copies an extension's page templates (Go `html/template` files) into `pb_templates/<set>/`, where your edits replace the built-in pages. It asks before overwriting files you changed (`--force` doesn't ask). Provides templates/v1.
