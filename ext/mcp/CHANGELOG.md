# MCP changelog

Changes to `ext/mcp`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

- A connection holding a scope for some values only (`notes=abc123`, new in API Tokens) sees the tools that scope covers.

## PocketOne v0.0.1-alpha.2 (2026-11-16)

### Changed

- Every tool is one REST call run as the caller; connections stay signed in; the consent page can limit an agent to some scopes.
