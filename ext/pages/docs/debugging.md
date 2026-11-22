---
title: Debugging in Pages
description: Dev mode, debug in +config.ts, and logging with ctx.log.
---

# Debugging in Pages

## Dev mode

Run `pocketone` with `--dev`:

```bash
pocketone serve --dev
```

Dev mode turns on:

- detailed error pages (message, file and line, trimmed stack; see [Errors](errors.md));
- `ctx.log.debug` output.

## `debug: true`

To get the same in production, set `debug: true` in `+config.ts`:

```ts
module.exports = {
  debug: true,
}
```

Turn it off again when you're done. Detailed error pages show error messages, file paths and stack traces to anyone who triggers an error.

## Logging

`ctx.log` writes to the app's log:

```ts
ctx.log.debug('loaded product', ctx.params.id)  // only in dev mode or with debug: true
ctx.log.info('order placed')
ctx.log.warn('stock is low')
ctx.log.error('payment failed')
```

See [log](ctx/log.md).

## Template changes

Templates are cached by content hash and the route table is rebuilt on any file change, in every mode. If a change doesn't show, check the app's log for a startup error (bad file name, route conflict): the previous route table keeps serving until the error is fixed.
