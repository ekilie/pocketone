# Tokens changelog

Changes to `ext/tokens`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

### Upgrading

- `pocketone oauth eject` is gone: run `pocketone template eject oauth`. The OAuth pages are now Go templates in `pb_templates/oauth/` (`signin.tpl`, `head.tpl`, ...); copies in `pb_pages/_private/oauth/` are no longer read, so carry your edits over to the new files.
- Go API: `tokens.Eject`, `tokens.ExistsError`, `tokens.EjectSubdir` and `OAuthServer.PagesDir()` are gone; the pages are the `tokens.TemplateSet` ("oauth") template set of templates/v1.

### Changed

- The OAuth pages are one set of Go templates, embedded in the binary and rendered through templates/v1 (ext/templates), so the built-in pages and the ones you eject are the same files.

## PocketOne v0.0.1-alpha.6 (2026-11-16)

### Upgrading

- The CLI's built-in OAuth client is now `pocketone-cli` (was `pocketbase-cli`). Browser logins made before this release can't renew once the remote is upgraded: run `pocketone login <remote>` again when one expires.

### Changed

- The CLI's OAuth client id, the CLI's User-Agent and the default deploy source are now `pocketone-cli`, matching the "pocketone CLI" name on the sign-in pages.

## PocketOne v0.0.1-alpha.2 (2026-11-16)

### Added

- Custom scopes for apps and extensions (Go guards, `$tokens` in JS, scope headers for collection rules), and any auth record can own tokens. OAuth sign-in goes through PocketBase's own auth endpoints.
