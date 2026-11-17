# Pages changelog

Changes to `ext/pages`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

### Fixed

- A long-running app no longer runs out of JS memory serving pages. Each page request stayed in the VM (its `rt` object and about 55 handles), so after a few thousand renders every page failed with `InternalError: out of memory` until the app restarted. Requests now free what they leave behind when they end.

### Changed

- The PocketPages docs and blog moved from pocketpages.dev into this folder (`site/`, `blog/`) and are now on
  pocketone.dev under [/docs/extensions/pages](https://pocketone.dev/docs/extensions/pages); the minimal starter links
  there.
- Compiled templates and modules are kept in the `pages.compiled` cache bucket (cache/v1) instead of a map that only grew: memory is capped (32 MiB per app) and old versions are evicted, and on a multisite host instances serving the same files share one compiled copy. Set its caps under `cache.buckets.pages.compiled` in `pocketone.yaml`.

### Upgrading

- Go API: `Pages.RenderTemplate` is gone. Its one user, the OAuth pages, moved to Go templates in ext/templates (templates/v1).

## PocketOne v0.0.1-alpha.6 (2026-11-16)

### Changed

- Runs on `ext/jsvm`. Templates can include each other at most 32 deep (was 64). A page that runs away now fails with the engine's "interrupted" or "out of memory" error.

## PocketOne v0.0.1-alpha.3 (2026-11-16)

### Upgrading

- A `+config.js` must export an object (`module.exports = { ... }`); the 0.x function form is now an error instead of being ignored.
- JS plugins that still use 0.x hooks (`onExtendContextApi`, `onRender`, `onResponse`, `handles`) are now an error that names the v1 hook to use (`extendContext`, `transform`).
- Move any `*.pb.js` / `*.pb.ts` file out of `pb_pages` into `pb_hooks`; in `pb_pages` it's now a startup error and isn't served.

### Changed

- A plugin name in `+config.js` is required first (a `_private/` file or package, such as the Datastar add-on as `'datastar'`), then a built-in.
- A bare `require('pkg')` looks in `_private/` folders first, then `node_modules` in `pb_pages`, then `pb_hooks/node_modules` and the app folder's `node_modules`; a miss lists the folders searched.
- A page renders for any HTTP method, as in 0.x.
- An `Authorization` header login wins over the `pb_auth` cookie, as in 0.x.
- Sign-in refuses accounts that need MFA.
- `pocketone pages create` and every starter README show the `superuser upsert` step before the first `serve`.
- The daisyui starters' Tailwind input is `-app.tailwind.css`, so it isn't served.
- The Datastar add-on follows the Datastar 1.0.5 SDK spec (`viewTransitionSelector`, `namespace`, signals from the query for DELETE) and has 0.x's helpers back.

### Fixed

- A circular `require` no longer crashes the process; it gets the exports built so far, as in Node.
- A new page is served after a build that still has errors, instead of 404 until restart.
- `__dirname` and `__filename` are absolute paths.
- `/.well-known/` is served again (ACME challenges, `security.txt`, app links).
- `include('nav.main')` finds `nav.main.ejs`, and 0.x's `include('/blog/_private/x')` form works.
- A layout that prints `<%- slot %>` shows the page instead of the slot function's source.
- A `+page.js` with no page beside it serves its folder's URL.
- `ctx.res.redirect(location, status)` takes a number and `ctx.res.cookie` takes `expires`.
- EJS templates get `locals`, as in EJS and 0.x.
