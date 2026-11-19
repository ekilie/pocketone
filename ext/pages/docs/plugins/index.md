---
title: Plugins
description: Built-in plugins (auth, realtime) and custom plugins such as the Datastar add-on, turned on in +config.ts.
---

# Plugins

Plugins add features to `ctx` and hook into each request. Turn them on in `+config.ts`:

```ts
// pb_pages/+config.ts
module.exports = {
  plugins: ['auth', 'realtime', 'datastar'],
}
```

Each entry is one of:

- **A name in quotes.** It's loaded with `require`, which looks in this order:
  1. a `_private/` folder in `pb_pages` (`'datastar'` finds `_private/datastar.js`)
  2. `pb_pages/node_modules`, for a package from your `package.json`
  3. the built-in plugins, `'auth'` and `'realtime'`

  A path such as `'./plugins/my-plugin.ts'` works too.
- **A plugin factory**, usually `require('./_private/my-plugin.ts')`. Writing the `require` yourself with a path skips
  the lookup above and loads exactly that file, which is how you pick one copy over another: `'datastar'` would find
  `_private/datastar.js` first, while `require('./node_modules/datastar')` always loads the package.

Datastar isn't built in. It's an add-on file you copy into `pb_pages/_private/` and list by name.

ETS templates and Markdown are always on; they aren't plugins.

## Built-in plugins

| Name | Adds |
|---|---|
| [`auth`](auth.md) | Cookie sessions on PocketBase auth: sets `ctx.auth` from the `pb_auth` cookie and adds `ctx.session.*` sign-in methods. |
| [`realtime`](realtime.md) | `ctx.realtime.send()` to PocketBase realtime subscribers. |

## Custom plugins

A custom plugin is a module that exports a factory returning hooks. List it by name, or `require()` it yourself, which you
need for options (`{ factory, ...options }`):

```ts
module.exports = {
  plugins: ['auth', 'my-plugin', { factory: require('./_private/other-plugin.ts'), color: 'blue' }],
}
```

See [Plugin Authoring](authoring.md).

| Add-on | Adds |
|---|---|
| [Datastar](datastar.md) | `ctx.datastar.*` helpers for [Datastar](https://data-star.dev). Copy [`addons/datastar.js`](https://github.com/pocketonehq/pocketone/blob/main/ext/pages/addons/datastar.js) into `pb_pages/_private/`; the `datastar` starter kit already has it. |

## Order

Plugins' hooks run in the order they're listed, and `onRequest` hooks run before any middleware.
