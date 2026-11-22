# Search changelog

Changes to `ext/search`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).

## Unreleased

### Added

- Search buckets (`ext/search`, contract `search/v1`): named buckets of content an app searches by meaning. An app makes the bucket it needs with `$search.ensure({ name, description })`; admins fill it on the dashboard's new **Search** screen with sources, each one tracked unit: files, folders and zips they drop, pasted text, web pages and Git repositories (read again every hour, day or week), and collections (kept in sync as records are saved and deleted). Adding a source of the same name replaces it; unchanged files cost nothing, missing ones are removed, and identical content anywhere in the app is embedded once. Every bucket uses the app's one embedding model, picked automatically from Settings → LLM and changeable on the screen (everything re-embeds in the background, with progress shown). `await $search.searchAsync({ query, buckets, viewer })` ranks across buckets by meaning and keywords together; `viewer` applies collections' view rules. `enrich` on `$llm` calls adds the best passages to the prompt.
