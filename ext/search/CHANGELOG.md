# Search changelog

Changes to `ext/search`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- Search buckets (`ext/search`, contract `search/v1`): named buckets of content an app searches by meaning and keywords together. An app makes the bucket it needs with `$search.ensure({ name, description })` and searches it with `await $search.searchAsync({ bucket, query })`; admins load files, folders or pasted text on the dashboard's new **Search** screen and try searches there. Markdown is split at headings, code and text at blank lines; chunks are embedded in the background through llm.embed/v1 (only changed chunks when a document changes), and buckets without a model are searched by keywords.
