---
title: Plugins
description: Built-in plugins (auth, realtime) and JS plugins such as the Datastar add-on, turned on in +config.js.
---

# Plugins

Plugins add features to `ctx` and hook into each request. Turn them on in `+config.js`:

```js
// pb_pages/+config.js
module.exports = {
  plugins: ['auth', 'realtime', './_private/datastar.js'],
}
```

Each entry is one of:

- **A module name in quotes.** It's loaded with `require`, so it can be a package from your `package.json`
  (`'my-plugin'`, found in `pb_pages/node_modules`) or a file in your app (`'./_private/my-plugin.js'`).
- **A built-in name**, `'auth'` or `'realtime'`. A string is a built-in only when no module by that name is found.
- **A plugin factory**, usually `require('./_private/my-plugin.js')`. Writing the `require` yourself and writing just
  the string do the same thing.

Datastar isn't built in. It's an add-on file you copy into `pb_pages/_private/` and list by path.

EJS and Markdown are always on; they aren't plugins.

## Built-in plugins

| Name | Adds |
|---|---|
| [`auth`](/docs/plugins/auth) | Cookie sessions on PocketBase auth: sets `ctx.auth` from the `pb_auth` cookie and adds `ctx.session.*` sign-in methods. |
| [`realtime`](/docs/plugins/realtime) | `ctx.realtime.send()` to PocketBase realtime subscribers. |

## JS plugins

A JS plugin is a module that exports a factory returning hooks. List it by name, or `require()` it yourself, which you
need for options (`{ factory, ...options }`):

```js
module.exports = {
  plugins: ['auth', 'my-plugin', { factory: require('./_private/other-plugin.js'), color: 'blue' }],
}
```

See [Plugin Authoring](/docs/plugins/authoring).

| Add-on | Adds |
|---|---|
| [Datastar](/docs/plugins/datastar) | `ctx.datastar.*` helpers for [Datastar](https://data-star.dev). Copy [`addons/datastar.js`](https://github.com/pockethost/pocketone/blob/main/ext/pages/addons/datastar.js) into `pb_pages/_private/`; the `datastar` starter kit already has it. |

## Order

Plugins' hooks run in the order they're listed, and `onRequest` hooks run before any middleware.
