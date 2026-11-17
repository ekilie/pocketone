# JSVM changelog

Changes to `ext/jsvm`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

## 0.0.2 (2026-11-17)

### Added

- Go API: `js.Release(v)` (`ext/extkit/js`) frees a JS value now instead of when Go collects its wrapper, for values a Go closure held by the VM can reach (otherwise a cycle neither GC frees). `(*jsvm.VM).GC()` runs a Go collection, the frees it posts and QuickJS's cycle collector, for tests and diagnostics.

### Fixed

- A `$app` database call at a hook file's top level, which fails because hook files load before the database opens (as in PocketBase), now says so and points to `onBootstrap`, instead of a bare nil pointer error.
- The README no longer says a call that waits lets other requests run as well without `.async`; under load a plain `sleep(100)` served 19 requests a second against 481 with `await sleep.async(100)`.

## PocketOne v0.0.1-alpha.6 (2026-11-16)

### Upgrading

- PocketBase's `plugins/jsvm` is upstream's again: `Config.Extensions`, `jsvm.Run` and `jsvm.ScriptPath` are gone from it. Use `ext/jsvm` (its extension methods and `js.Run`) instead. `ext/jserrors` is removed; `ext/jsvm` reports real file positions.
- Go code that binds JS takes `js.Runtime` (`github.com/pockethost/pocketone/ext/extkit/js`) instead of `*goja.Runtime`: `extkit.JSBinder.BindJS(app, rt)`, `extkit.BindJS(app, rt, ...)`, `extkit.AppSet.BindJS`, Pocker's `InitJS(inst, app, rt)`, `mcp.BindJS(app, rt)`, `tokens.BindJS(app, rt)`, `bulkmail.BindJS(app, rt)`. `extkit.JSContracts(app)` no longer takes a runner. `standard.Set.Register` takes an `ext/jsvm` config.

### Added

- `pb_data/types.d.ts` now covers PocketOne: generated from PocketOne's core (its extra hooks, e.g. `onFilesystemCreate`), with typed `.async` on every Go function, `require`, `process`, `Buffer`, `import.meta`, and the API of each extension on in the app (`$mcp`, `$tokens`, `$bulkmail`, `onContracts`). Extensions add theirs with a `TypeScript()` method.
- TypeScript: `.ts`, `.mts` and `.cts` hooks (`*.pb.ts`, `*.pb.mts`, `*.pb.cts`), migrations and modules run directly, with types stripped by esbuild and error positions in the `.ts` source. `.ts` and `.mts` are ES modules, `.cts` is CommonJS; `import "./x.js"` finds `x.ts`. No runtime type checking: use `tsc --noEmit` with `pb_data/types.d.ts`.
- ES modules: `pb_hooks/*.pb.mjs` hook files, `pb_migrations/*.mjs` migrations and `.mjs` modules use `import`/`export` with top-level `await` and `import.meta`. `*.pb.js` and the new `*.pb.cjs` stay CommonJS scripts with `require()`. `import` and `require()` share one resolver (relative paths, `node_modules`, `package.json` `exports` conditions) and one instance per module; `require()` of an ES module without top-level `await` returns its namespace, as in Node 22.
- `jsvm: goja` under `app:` in `pocketone.yaml`, or `--jsvm goja`, runs a single app on PocketBase's own JS engine for pure PocketBase compatibility (pages, `$mcp`, `$tokens`, `$bulkmail` and JS contracts are off; the startup lines say so). The default is `quickjs`.
- New `ext/jsvm` extension, PocketOne's JS engine: one QuickJS VM per app with real globals, async handlers and `.async` Go calls, and hard per-app memory and time limits. It runs PocketBase's JS API unchanged, and is pure Go (no cgo), so `CGO_ENABLED=0` builds and cross-compiles work as with stock PocketBase.

### Changed

- The `pocketone` binary and every Pocker instance run JS on `ext/jsvm` (QuickJS, one VM per app) instead of goja. Hooks, routes and migrations run as before; hook files now share one global scope, handlers can be `async`, and errors name `pb_hooks/<file>:<line>:<col>` on their own (`ext/jserrors` is no longer used). `--hooksPool` is accepted and ignored.

### Fixed

- A hook can no longer pin a CPU with an Array method on a huge sparse array or array-like (`new Array(1e9).join("x")`, `[].indexOf.call({ length: 2 ** 53 - 1 }, 1)`), which QuickJS ran without checking the time limit. Array methods that walk every index, and passing an array to a Go function, now throw a `RangeError` past `MemoryLimit / 16` elements (4,194,304 at the default 64 MiB), a length no dense array fits in that memory.
