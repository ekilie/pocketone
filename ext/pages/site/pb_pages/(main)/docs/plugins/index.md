---
title: Plugins
description: Built-in plugins (auth, realtime) and JS plugins such as the Datastar add-on, turned on in +config.js.
---

# Plugins

Plugins add features to `ctx` and hook into each request. Turn them on in `+config.js`:

```js
// pb_pages/+config.js
module.exports = {
  plugins: ['auth', 'realtime', require('./_private/datastar.js')],
}
```

There are two kinds of entry:

- **A name in quotes** (`'auth'`, `'realtime'`) turns on a plugin built into PocketPages. Only these two exist; any other
  name is a startup error.
- **`require('./_private/some-plugin.js')`** loads a JS plugin from a file in your app. Datastar works this way: it's an
  add-on file you copy into `pb_pages/_private/`, not a built-in.

EJS and Markdown are always on; they aren't plugins.

## Built-in plugins

| Name | Adds |
|---|---|
| [`auth`](/docs/plugins/auth) | Cookie sessions on PocketBase auth: sets `ctx.auth` from the `pb_auth` cookie and adds `ctx.session.*` sign-in methods. |
| [`realtime`](/docs/plugins/realtime) | `ctx.realtime.send()` to PocketBase realtime subscribers. |

## JS plugins

A JS plugin is a file that exports a factory returning hooks. `require()` it and add it to `plugins` directly, or as
`{ factory, ...options }`:

```js
module.exports = {
  plugins: ['auth', require('./_private/my-plugin.js')],
}
```

See [Plugin Authoring](/docs/plugins/authoring).

| Add-on | Adds |
|---|---|
| [Datastar](/docs/plugins/datastar) | `ctx.datastar.*` helpers for [Datastar](https://data-star.dev). Copy [`addons/datastar.js`](https://github.com/pockethost/pocketone/blob/main/ext/pages/addons/datastar.js) into `pb_pages/_private/`; the `datastar` starter kit already has it. |

## Order

Plugins' hooks run in the order they're listed, and `onRequest` hooks run before any middleware.
