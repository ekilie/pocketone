---
title: Configuration with +config.ts
description: Turn on built-in plugins, add custom plugins, and enable debug output from pb_pages/+config.ts.
---

# Configuration with `+config.ts`

Create `pb_pages/+config.ts` to configure Pages for the app. It's only allowed at the top of `pb_pages`; a `+config.ts` in a subfolder is an error at startup.

```ts
// pb_pages/+config.ts
module.exports = {
  plugins: ['auth', 'realtime', 'datastar'],
  debug: false,
}
```

## Options

| Key | Default | |
|---|---|---|
| `plugins` | `[]` | Built-in plugins by name, or custom plugin factories. ETS templates and Markdown are always on. |
| `debug` | `false` | Detailed error pages and `ctx.log.debug` output even outside dev mode. |

`+config.ts` must export an object; exporting a function (0.x's `(api) => ({...})` form) is an error.

`+config.ts` runs on each page request, so these aren't caught at startup: unknown keys, names that are neither a
module nor a built-in, or a plugin listed twice make every page answer with an error (a 500; dev mode shows the
reason on the page), and the reason goes to the log.

## Plugins

Each entry in `plugins` is one of:

- **A name**, loaded with `require`: a `_private/` file (`'datastar'` for the [Datastar add-on](plugins/datastar.md)),
  then a package in `node_modules`, then a built-in (`'auth'` or `'realtime'`). It must export a plugin factory. See
  [Plugins](plugins/index.md).
- **A custom plugin factory**: a function that returns hooks, such as `require('./_private/my-plugin.ts')`. A `require`
  with a path loads exactly that file, skipping the name lookup, so it's how you choose one copy over another.
- **An object with a factory and options**: `{ factory, ...options }`. The object is passed to the factory as its `config`.

```ts
module.exports = {
  plugins: [
    'auth',
    'realtime',
    { factory: require('./_private/banner.ts'), text: 'Hello' },
  ],
}
```

Plugin hooks run in the order the plugins are listed. See [Plugin Authoring](plugins/authoring.md).

## Debug

`debug: true` turns on what dev mode (`--dev`) turns on: detailed error pages and `ctx.log.debug` output. See [Debugging](debugging.md).
