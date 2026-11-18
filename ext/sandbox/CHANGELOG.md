# Sandbox changelog

Changes to `ext/sandbox`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

- `UninstallEgressPolicy()` undoes `InstallEgressPolicy`, giving Go's default HTTP transport back its own dialer and proxy (for tests that start a Pocker host in-process).

## PocketOne v0.0.1-alpha.6 (2026-11-16)

### Upgrading

- `MaxCallStackSize` and `MaxConcurrent` are gone (the engine caps call depth with `jsvm.Config.VM.StackSize`; set `jsvm.Config.VM.MaxConcurrent`). `Sandbox.Limits()` gives the execution and sleep limits for `jsvm.Config.VM`.

### Changed

- Each instance's JS has a hard memory cap (64 MiB: JS heap plus Go objects it holds); gradual growth is now limited per instance, not only one-shot allocations. Process-reaching Go methods are also hidden when an app type embeds `*core.BaseApp`.

### Fixed

- A hook can no longer pin a CPU with an Array method on a huge sparse array or array-like (`new Array(1e9).join("x")`, `[].indexOf.call({ length: 2 ** 53 - 1 }, 1)`), which QuickJS ran without checking the time limit. Array methods that walk every index, and passing an array to a Go function, now throw a `RangeError` past `MemoryLimit / 16` elements (4,194,304 at the default 64 MiB), a length no dense array fits in that memory.
