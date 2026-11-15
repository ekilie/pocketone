---
title: Configuration with +config.js
description: Turn on built-in plugins, add JS plugins, and enable debug output from pb_pages/+config.js.
---

# Configuration with `+config.js`

Create `pb_pages/+config.js` to configure PocketPages for the app:

```js
// pb_pages/+config.js
module.exports = {
  plugins: ['auth', 'realtime', 'datastar'],
  debug: false,
}
```

## Options

| Key | Default | |
|---|---|---|
| `plugins` | `[]` | Built-in plugins by name, or JS plugin factories. EJS and Markdown are always on. |
| `debug` | `false` | Detailed error pages and `ctx.log.debug` output even outside dev mode. |

Unknown keys, names that are neither a module nor a built-in, and duplicates are startup errors.

## Plugins

Each entry in `plugins` is one of:

- **A name**, loaded with `require`: a `_private/` file (`'datastar'` for the [Datastar add-on](/docs/plugins/datastar)),
  then a package in `node_modules`, then a built-in (`'auth'` or `'realtime'`). It must export a plugin factory. See
  [Plugins](/docs/plugins).
- **A JS plugin factory**: a function that returns hooks, such as `require('./_private/my-plugin.js')`.
- **An object with a factory and options**: `{ factory, ...options }`. The object is passed to the factory as its `config`.

```js
module.exports = {
  plugins: [
    'auth',
    'realtime',
    { factory: require('./_private/banner.js'), text: 'Hello' },
  ],
}
```

Plugin hooks run in the order the plugins are listed. See [Plugin Authoring](/docs/plugins/authoring).

## Debug

`debug: true` turns on what dev mode (`--dev`) turns on: detailed error pages and `ctx.log.debug` output. See [Debugging](/docs/debugging).
