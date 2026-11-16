# PocketOne changelog

PocketOne's own changes, newest first. Upstream PocketBase's changes are in [CHANGELOG.md](../CHANGELOG.md) at the
repo root; each release here says which PocketBase version it's built on.

## How this file is kept

- **Every commit to `main` with a user-visible change adds its line under `## Unreleased` in the same commit:** a
  feature, a fix, a changed behavior or default, a removal, or a new upstream PocketBase version. Docs-only and
  internal changes don't need one.
- **Group lines under these headings, in this order, and leave out empty ones:**
  - **Upgrading:** what someone on the previous release must do, as steps. Required for anything breaking.
  - **Added**
  - **Changed**
  - **Fixed**
  - **PocketBase:** the upstream version this is built on, when it changes.
- **Write for someone running the binary:** what changed and what to do, with the command, flag or config key. Start
  extension-specific lines with the extension's name, for example "**pages:**".
- **One line per change.** GitHub shows every newline as a line break, so don't hard-wrap lines.
- **Cutting a release:**
  1. Rename `## Unreleased` to `## vX.Y.Z (YYYY-MM-DD)`, put a new empty `## Unreleased` above it, and commit.
  2. Run `examples/pocketone/release-notes.sh vX.Y.Z > /tmp/notes.md`.
  3. Run `goreleaser release --config examples/pocketone/.goreleaser.yaml --release-notes /tmp/notes.md`.

  The release header adds the install and upgrade steps on top. Release notes are this section and nothing else.

## Unreleased

### Upgrading

- **jsvm:** PocketBase's `plugins/jsvm` is upstream's again: `Config.Extensions`, `jsvm.Run` and `jsvm.ScriptPath` are gone from it. Use `ext/jsvm` (its extension methods and `js.Run`) instead. `ext/jserrors` is removed; `ext/jsvm` reports real file positions.
- **jsvm:** Go code that binds JS takes `js.Runtime` (`github.com/pockethost/pocketone/ext/extkit/js`) instead of `*goja.Runtime`: `extkit.JSBinder.BindJS(app, rt)`, `extkit.BindJS(app, rt, ...)`, `extkit.AppSet.BindJS`, Pocker's `InitJS(inst, app, rt)`, `mcp.BindJS(app, rt)`, `tokens.BindJS(app, rt)`, `bulkmail.BindJS(app, rt)`. `extkit.JSContracts(app)` no longer takes a runner. `standard.Set.Register` takes an `ext/jsvm` config.
- **sandbox:** `MaxCallStackSize` and `MaxConcurrent` are gone (the engine caps call depth with `jsvm.Config.VM.StackSize`; set `jsvm.Config.VM.MaxConcurrent`). `Sandbox.Limits()` gives the execution and sleep limits for `jsvm.Config.VM`.
- **bundle:** `ext/jsembed` is now `ext/bundle` and runs on the `ext/jsvm` engine. Import `github.com/pockethost/pocketone/ext/bundle`, register `ext/jsvm` instead of `plugins/jsvm`, and pass `bundle.Hooks(...)` / `bundle.Migrations(...)` in `jsvm.Config.Extensions`. A `require()` that isn't found now says "Cannot find module".

### Added

- **jsvm:** `pb_data/types.d.ts` now covers PocketOne: generated from PocketOne's core (its extra hooks, e.g. `onFilesystemCreate`), with typed `.async` on every Go function, `require`, `process`, `Buffer`, `import.meta`, and the API of each extension on in the app (`$mcp`, `$tokens`, `$bulkmail`, `onContracts`). Extensions add theirs with a `TypeScript()` method.
- **jsvm:** TypeScript: `.ts`, `.mts` and `.cts` hooks (`*.pb.ts`, `*.pb.mts`, `*.pb.cts`), migrations and modules run directly, with types stripped by esbuild and error positions in the `.ts` source. `.ts` and `.mts` are ES modules, `.cts` is CommonJS; `import "./x.js"` finds `x.ts`. No runtime type checking: use `tsc --noEmit` with `pb_data/types.d.ts`.
- **jsvm:** ES modules: `pb_hooks/*.pb.mjs` hook files, `pb_migrations/*.mjs` migrations and `.mjs` modules use `import`/`export` with top-level `await` and `import.meta`. `*.pb.js` and the new `*.pb.cjs` stay CommonJS scripts with `require()`. `import` and `require()` share one resolver (relative paths, `node_modules`, `package.json` `exports` conditions) and one instance per module; `require()` of an ES module without top-level `await` returns its namespace, as in Node 22.

- **jsvm:** `jsvm: goja` under `app:` in `pocketone.yaml`, or `--jsvm goja`, runs a single app on PocketBase's own JS engine for pure PocketBase compatibility (pages, `$mcp`, `$tokens`, `$bulkmail` and JS contracts are off; the startup lines say so). The default is `quickjs`.
- **bundle:** `bundle.Public(app, fsys, indexFallback)` serves public files embedded in the binary, like `pb_public`.
- **jsvm:** new `ext/jsvm` extension, PocketOne's JS engine: one QuickJS VM per app with real globals, async handlers and `.async` Go calls, and hard per-app memory and time limits. It runs PocketBase's JS API unchanged, and is pure Go (no cgo), so `CGO_ENABLED=0` builds and cross-compiles work as with stock PocketBase.

### Changed

- A request with an unknown or expired `Authorization` token now gets a 401 instead of going on as a guest (PocketBase v0.42.0). Clients that send a stale token should drop it and sign in again. `/mcp` keeps answering with its own sign-in challenge, so MCP clients still re-authenticate.
- **jsvm:** the `pocketone` binary and every Pocker instance run JS on `ext/jsvm` (QuickJS, one VM per app) instead of goja. Hooks, routes and migrations run as before; hook files now share one global scope, handlers can be `async`, and errors name `pb_hooks/<file>:<line>:<col>` on their own (`ext/jserrors` is no longer used). `--hooksPool` is accepted and ignored.
- **sandbox, pocker:** each instance's JS has a hard memory cap (64 MiB: JS heap plus Go objects it holds); gradual growth is now limited per instance, not only one-shot allocations. Process-reaching Go methods are also hidden when an app type embeds `*core.BaseApp`.
- **contracts:** contract providers and `on<Contract><Op>` handlers written in pb_hooks can use the file's top-level variables.
- **pages:** runs on `ext/jsvm`. Templates can include each other at most 32 deep (was 64). A page that runs away now fails with the engine's "interrupted" or "out of memory" error.

### Fixed

- A crash or data race when an app stops its cron scheduler (on shutdown or restart), a PocketBase v0.42 regression; fixed in core until upstream has it.

### PocketBase

- Built on PocketBase v0.42.2 (was v0.41.1). See its [changelog](../CHANGELOG.md) for v0.42.0 to v0.42.2.

## v0.0.1-alpha.4 (2026-11-16)

### Upgrading

- **pocker:** if you build your own host on Pocker, set `Standard: standard.New()` on `pocker.Server`, `pocker.Provider` or `pocker.ManagerOptions`. Pocker now refuses to start instances without the standard set, so every instance has tokens, OAuth sign-in, MCP, pages and the rest. Drop standard members (pages, pitr, settings, ...) from `Extensions`; configure them on the set instead. The stock `pocketone` binary already does this.

### Changed

- **pocker:** every instance gets the standard extensions; there's no way to start one without them.

## v0.0.1-alpha.3 (2026-11-16)

### Upgrading

- **pages:** a `+config.js` must export an object (`module.exports = { ... }`); the 0.x function form is now an error instead of being ignored.
- **pages:** JS plugins that still use 0.x hooks (`onExtendContextApi`, `onRender`, `onResponse`, `handles`) are now an error that names the v1 hook to use (`extendContext`, `transform`).
- **pages:** move any `*.pb.js` / `*.pb.ts` file out of `pb_pages` into `pb_hooks`; in `pb_pages` it's now a startup error and isn't served.

### Added

- `pocketone --version` prints the PocketBase version it's built on: `0.0.2 (PocketBase 0.41.1)`.
- **docker:** a host with its own accounts decides who may call `remote/v1` (status, restart, logs) for its containers with `docker.Provider.RemoteAuth`; without it, the instance's own superuser login is checked as before. `remote/v1` now lives on the host's contract scope, so `pocketone remote whoami` also lists the host's other contracts for the instance (a host's `deploy/v1`).

### Changed

- **pages:** a plugin name in `+config.js` is required first (a `_private/` file or package, such as the Datastar add-on as `'datastar'`), then a built-in.
- **pages:** a bare `require('pkg')` looks in `_private/` folders first, then `node_modules` in `pb_pages`, then `pb_hooks/node_modules` and the app folder's `node_modules`; a miss lists the folders searched.
- **pages:** a page renders for any HTTP method, as in 0.x.
- The startup lines listing the standard extensions and contracts print when the app serves, not before every command (`--version`, `help`, `migrate`).
- **pages:** an `Authorization` header login wins over the `pb_auth` cookie, as in 0.x.
- **pages:** sign-in refuses accounts that need MFA.
- **pages:** `pocketone pages create` and every starter README show the `superuser upsert` step before the first `serve`.
- **pages:** the daisyui starters' Tailwind input is `-app.tailwind.css`, so it isn't served.
- **pages:** the Datastar add-on follows the Datastar 1.0.5 SDK spec (`viewTransitionSelector`, `namespace`, signals from the query for DELETE) and has 0.x's helpers back.

### Fixed

- **pages:** a circular `require` no longer crashes the process; it gets the exports built so far, as in Node.
- **pages:** a new page is served after a build that still has errors, instead of 404 until restart.
- **pages:** `__dirname` and `__filename` are absolute paths.
- **pages:** `/.well-known/` is served again (ACME challenges, `security.txt`, app links).
- **pages:** `include('nav.main')` finds `nav.main.ejs`, and 0.x's `include('/blog/_private/x')` form works.
- **pages:** a layout that prints `<%- slot %>` shows the page instead of the slot function's source.
- **pages:** a `+page.js` with no page beside it serves its folder's URL.
- **pages:** `ctx.res.redirect(location, status)` takes a number and `ctx.res.cookie` takes `expires`.
- **pages:** EJS templates get `locals`, as in EJS and 0.x.

## v0.0.1-alpha.2 (2026-11-16)

Built on PocketBase v0.41.1.

### Upgrading

- If you installed the old `v0.0.1`, reinstall (`brew reinstall pockethost/tap/pocketone`, or download the zip): `v0.0.1` became `v0.0.1-alpha.1`, and later alphas won't replace it on their own.

### Added

- **Extension contracts:** every extension API is a versioned contract, so extensions depend on what they need, not on each other by name.
- **Config:** `pocketone config --json` lists every section, key, type and default. The `pitr`, `autotls` and `s3redirect` sections set a single app's defaults; flags still win.
- **tokens:** custom scopes for apps and extensions (Go guards, `$tokens` in JS, scope headers for collection rules), and any auth record can own tokens. OAuth sign-in goes through PocketBase's own auth endpoints.
- **Dashboard:** the PocketOne logo, favicon and version in the superuser dashboard.
- Every extension's docs are on [pocketone.dev](https://pocketone.dev), now with search.

### Changed

- **`pocketone upgrade`** keeps every old executable as `pocketone-<version>` and follows prereleases: from one alpha to the next it just upgrades; from a stable release to a prerelease it asks first.
- **Config file is `pocketone.yaml`.**
- **mcp:** every tool is one REST call run as the caller; connections stay signed in; the consent page can limit an agent to some scopes.
- **deploy:** two-way `pocketone dev` sync, and a deploy never restarts the app on its own (`deploy --restart`, `POST /restart`, or `r` in dev).
- **Hooks:** in dev mode a broken `pb_hooks` file is skipped instead of stopping the app; a bad reload keeps the last good code.

### Fixed

- The hooks watcher's goroutine is stopped before bootstrap is cleared (a race in PocketBase's notify watcher).

## v0.0.1-alpha.1 (2026-11-16)

The first alpha, first published as `v0.0.1`. Built on PocketBase v0.41.1.

### Added

- Runs your existing `pb_data`, `pb_hooks` and `pb_migrations`.
- **pitr:** point-in-time restore (Litestream), from Settings → Backups or `pocketone pitr restore`.
- **pages** (formerly PocketPages): server-rendered pages from `pb_pages`; start with `pocketone pages create myapp`.
- **mcp** at `/mcp`, **Bulk Mail**, **Auto TLS**, **API tokens**, an **OAuth server**, **S3 redirects**, and `pocketone deploy | dev | pull | logs`.
- A dev-mode mail trap (`serve --dev`) at http://127.0.0.1:8025.
