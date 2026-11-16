# Pages changelog

Changes to `ext/pages`, newest first. A user-visible change adds its line under `## Unreleased` in the same
commit. Each PocketOne release raises this extension's `version:` when there's something under `## Unreleased` and
renames the section to that version ([Changelogs](../../docs/extensions.md#changelogs)).
Sections named after a PocketOne release are from before extensions had versions of their own.

## Unreleased

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
