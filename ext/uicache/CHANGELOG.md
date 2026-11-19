# UI Cache changelog

Changes to `ext/uicache`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

## 0.0.2 (2026-11-18)

- New: dashboard files that keep their name (`/_/extensions.js`, `libs/`, fonts, images, extension files) are no longer cached for 14 days, so browsers and CDNs pick up extension and PocketOne updates.

