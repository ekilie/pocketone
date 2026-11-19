---
title: API Routes and JSON
description: Serve JSON and other non-HTML responses from +api.js files and ctx.res.json.
---

# API Routes and JSON

A `+api.js` file serves its folder's URL without a template. Use it for JSON, feeds, webhooks and anything else that isn't an HTML page.

```js
// pb_pages/feeds/latest/+api.js
module.exports = {
  get(ctx) {
    const posts = ctx.app.findRecordsByFilter('posts', 'published = true', '-created', 20)
    return ctx.res.json(posts.map((p) => ({ id: p.id, title: p.getString('title') })))
  },
}
```

```ts
// pb_pages/feeds/latest/+api.ts
type PostSummary = { id: string; title: string }

export function get(ctx: any) {
  const posts = ctx.app.findRecordsByFilter('posts', 'published = true', '-created', 20)
  const out: PostSummary[] = posts.map((p: any) => ({ id: p.id, title: p.getString('title') }))
  return ctx.res.json(out)
}
```

A request to `/feeds/latest` returns:

```json
[{ "id": "abc123", "title": "Hello" }]
```

## How it works

- `+api.js` has the same exports as `+page.js`: `load`, `get`, `post`, `put`, `patch`, `delete`.
- There is no template, so a handler must send a response. Returning without one is a `500`.
- `[param]` folders work as for pages: `products/[id]/+api.js` serves `/products/123`.
- A folder can't have both a `+api.js` and an `index` page.
- Middleware runs for API routes too.
- `ctx.httpError()` thrown in a `+api.js` answers with JSON: `{ "status": 404, "message": "..." }`.
- Handlers can be `async`. The response is sent once the handler settles.

## Async handlers

```js
// pb_pages/stats/+api.js
module.exports = {
  async get(ctx) {
    const orders = await ctx.app.countRecords.async('orders')
    return ctx.res.json({ orders })
  },
}
```

```ts
// pb_pages/stats/+api.ts
export async function get(ctx: any) {
  const orders: number = await ctx.app.countRecords.async('orders')
  return ctx.res.json({ orders })
}
```

An async handler still has to send a response. Returning a value without calling `ctx.res.json` (or another
`ctx.res` method) is a `500`, as it is for a sync handler.

## Sending responses

| Call | Sends |
|---|---|
| `ctx.res.json(data)` / `ctx.res.json(status, data)` | JSON |
| `ctx.res.text(text)` / `ctx.res.text(status, text)` | Plain text |
| `ctx.res.html(html)` / `ctx.res.html(status, html)` | HTML as-is, without layouts |
| `ctx.res.file(path)` | A file from `pb_pages` |
| `ctx.res.redirect(location)` | A redirect |
| `ctx.res.sse()` | A `text/event-stream` |

See [res](ctx/res.md).

## JSON from a page

JSON is only sent through `ctx.res.json` (or a `+api.js`). Template output is never parsed as JSON, even if it looks like JSON. A page's `+page.js` can still answer with JSON for one method:

```js
// pb_pages/products/[id]/+page.js
module.exports = {
  post(ctx) {
    const product = ctx.app.findRecordById('products', ctx.params.id)
    return ctx.res.json({ id: product.id, name: product.getString('name') })
  },
}
```

```ts
// pb_pages/products/[id]/+page.ts
export function post(ctx: any) {
  const product = ctx.app.findRecordById('products', ctx.params.id)
  return ctx.res.json({ id: product.id, name: product.getString('name') })
}
```

## Keep `/api/` free

`/api/` belongs to PocketBase. Pages never see it, and a `pb_pages/api/` folder is an error at startup. Put your routes somewhere else, such as `feeds/`, `xapi/` or `hooks/`.
