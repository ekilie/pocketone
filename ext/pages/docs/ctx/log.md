---
title: ctx.log
description: Write to the app's log with debug, info, warn and error.
---

# `ctx.log`

Writes to the app's log.

```ts
ctx.log.debug('cart', ctx.data.cart)
ctx.log.info('order placed')
ctx.log.warn('stock is low')
ctx.log.error('payment failed')
```

`debug` only shows in dev mode (`--dev`) or with `debug: true` in `+config.ts`. See [Debugging](../debugging.md).
