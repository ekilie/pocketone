# Pocker changelog

Changes to `ext/pocker`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

### Changed

- A `pb_hooks` change restarts the instance like PocketBase's watcher restarts a process: the running app stops, then a fresh one starts, so two apps never have the instance's folder open (migrations and bootstrap code no longer run beside a live app). In production a broken hook file now leaves the instance down (502 naming the file) until it's fixed, instead of the previous code serving on; dev mode still skips the broken file.

## PocketOne v0.0.1-alpha.6 (2026-11-16)

### Changed

- Each instance's JS has a hard memory cap (64 MiB: JS heap plus Go objects it holds); gradual growth is now limited per instance, not only one-shot allocations. Process-reaching Go methods are also hidden when an app type embeds `*core.BaseApp`.

## PocketOne v0.0.1-alpha.4 (2026-11-16)

### Upgrading

- If you build your own host on Pocker, set `Standard: standard.New()` on `pocker.Server`, `pocker.Provider` or `pocker.ManagerOptions`. Pocker now refuses to start instances without the standard set, so every instance has tokens, OAuth sign-in, MCP, pages and the rest. Drop standard members (pages, pitr, settings, ...) from `Extensions`; configure them on the set instead. The stock `pocketone` binary already does this.

### Changed

- Every instance gets the standard extensions; there's no way to start one without them.
