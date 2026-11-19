---
title: Errors
description: How PocketPages turns thrown errors into responses, ctx.httpError(), and custom +error and +404 pages.
---

# Errors

## Thrown errors

Anything thrown in middleware, a handler or a template becomes a `500`.

- **Dev mode or `debug: true`:** an HTML error page with the message, the file and line (template lines map to the `.ejs` or `.md` source), and the stack trimmed to `pb_pages`-relative paths.
- **Otherwise:** a plain "Something went wrong" page.

The full error always goes to the app's log.

## `ctx.httpError(status, message)`

Throw `ctx.httpError()` to answer with a specific status:

```js
// pb_pages/products/[id]/+page.js
module.exports = {
  load(ctx) {
    let product
    try {
      product = ctx.app.findFirstRecordByData('products', 'slug', ctx.params.id)
    } catch (e) {
      throw ctx.httpError(404, 'No such product') // findFirstRecordByData throws when nothing matches
    }
    ctx.data.product = product
  },
}
```

```ts
// pb_pages/products/[id]/+page.ts
export function load(ctx: any) {
  let product
  try {
    product = ctx.app.findFirstRecordByData('products', 'slug', ctx.params.id)
  } catch (e) {
    throw ctx.httpError(404, 'No such product') // findFirstRecordByData throws when nothing matches
  }
  ctx.data.product = product
}
```

- In a page, it sends that status with a plain page showing `message`.
- In a `+api.js`, it sends JSON: `{ "status": 404, "message": "No such product" }`.

## A custom error page

Create `pb_pages/+error.ejs`. It renders for every `ctx.httpError` and for every `500` outside dev mode, inside the root layout, with `ctx.error = { status, message }`:

```ejs
<!-- pb_pages/+error.ejs -->
<%% slot('title', () => { %>Error <%%= ctx.error.status %><%% }) %>
<h1><%%= ctx.error.status %></h1>
<p><%%= ctx.error.message %></p>
```

```tsx
// pb_pages/+error.tsx
export default function ErrorPage({ ctx, slot }: any) {
  const { status, message }: { status: number; message: string } = ctx.error
  slot('title', `Error ${status}`)
  return (
    <>
      <h1>{status}</h1>
      <p>{message}</p>
    </>
  )
}
```

`+error` and `+404` live only at the top of `pb_pages`. Either may be `.ejs`, `.jsx` or `.tsx`.

## A custom 404 page

Create `pb_pages/+404.ejs`. It renders inside the root layout, with status `404`, for a `GET` or `HEAD` that nothing answers: no page, no file in `pb_public` and no PocketBase route. It never renders for `/api/` or `/_/`. The root `+middleware.js` runs first, and page code can still change the status.

```ejs
<!-- pb_pages/+404.ejs -->
<%% slot('title', () => { %>Not found<%% }) %>
<h1>Not found</h1>
<p>Nothing lives at <%%= ctx.req.url.pathname %>.</p>
```

```tsx
// pb_pages/+404.tsx
export default function NotFound({ ctx, slot }: any) {
  slot('title', 'Not found')
  return (
    <>
      <h1>Not found</h1>
      <p>Nothing lives at {ctx.req.url.pathname}.</p>
    </>
  )
}
```
