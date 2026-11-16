# Standard Set changelog

Changes to `ext/standard`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- New member `Templates` (ext/templates): extensions' page templates, ejected to `pb_templates` to edit (`pocketone template eject oauth` for the sign-in screens). Turn it off per app with `templates: {enabled: false}`.
