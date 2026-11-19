---
title: Loading Data in PocketPages
description: Load data inline in templates or in a +page.js file with load and per-method handlers.
---

# Loading Data in PocketPages

You can load data right in a template, or in a `+page.js` file next to it.

## Inline data loading

The simplest way is a `<%% %>` block at the top of your template:

```ejs
<%%
  const products = ctx.app.findRecordsByFilter('products', 'active = true', 'name', 100)
%>

<h1>Products</h1>
<%% for (const product of products) { %>
  <div class="product">
    <h2><%%= product.getString('name') %></h2>
  </div>
<%% } %>
```

```tsx
export default function Products({ ctx }: any) {
  const products = ctx.app.findRecordsByFilter('products', 'active = true', 'name', 100)
  return (
    <>
      <h1>Products</h1>
      {products.map((product: any) => (
        <div class="product">
          <h2>{product.getString('name')}</h2>
        </div>
      ))}
    </>
  )
}
```

## Server script blocks

For better formatting support in editors, you can write `<script server<%- '>' %>` instead of `<%% %>`. They mean the same thing:

```ejs
<script server<%- '>' %>
  const products = ctx.app.findRecordsByFilter('products', 'active = true', 'name', 100)
  const categories = ctx.app.findAllRecords('categories')
</script>

<h1>Products by Category</h1>
<%% for (const category of categories) { %>
  <h2><%%= category.getString('name') %></h2>
<%% } %>
```

## Structured loading with `+page.js`

A `+page.js` file keeps data loading and form handling out of the template:

```js
// pb_pages/products/[id]/+page.js
module.exports = {
  load(ctx) {
    ctx.data.product = ctx.app.findRecordById('products', ctx.params.id)
  },
  post(ctx) {
    const { qty } = ctx.req.body()
    // ...
    return ctx.res.redirect(`/products/${ctx.params.id}`, { flash: 'Added to cart' })
  },
}
```

```ts
// pb_pages/products/[id]/+page.ts
export function load(ctx: any) {
  ctx.data.product = ctx.app.findRecordById('products', ctx.params.id)
}

export function post(ctx: any) {
  const { qty } = ctx.req.body()
  // ...
  return ctx.res.redirect(`/products/${ctx.params.id}`, { flash: 'Added to cart' })
}
```

```ejs
<!-- pb_pages/products/[id]/index.ejs -->
<h1><%%= data.product.getString('name') %></h1>
<%% const flash = ctx.flash() %>
<%% if (flash) { %><p><%%= flash %></p><%% } %>
```

```tsx
// pb_pages/products/[id]/index.tsx
export default function Product({ ctx, data }: any) {
  const flash = ctx.flash()
  return (
    <>
      <h1>{data.product.getString('name')}</h1>
      {flash && <p>{flash}</p>}
    </>
  )
}
```

### Exports

`+page.js` exports any of `load`, `get`, `post`, `put`, `patch` and `delete`.

1. `load` runs first, for every method.
2. Then the handler for the request method runs, if there is one.
3. Then the page renders.

A handler can set fields on `ctx.data`, or return a plain object. A returned object is shallow-assigned into `ctx.data` (`Object.assign`); there is no deep merge.

```js
module.exports = {
  load(ctx) {
    return { departments: ctx.app.findAllRecords('departments') }
  },
  get(ctx) {
    ctx.data.mode = 'view'
  },
}
```

If a handler sends a response (`ctx.res.redirect`, `json`, `html`, `text`, `file`, `sse`), the page doesn't render.

### Async loading

`load` and the method handlers can be `async`. PocketPages waits for them before it renders. Every Go function has an
`.async` twin that does its work outside the JS VM, so other requests keep running while it waits:

```js
// pb_pages/dashboard/+page.js
module.exports = {
  async load(ctx) {
    const [orders, users] = await Promise.all([
      ctx.app.countRecords.async('orders'),
      ctx.app.countRecords.async('users'),
    ])
    return { orders, users }
  },
}
```

```ts
// pb_pages/dashboard/+page.ts
export async function load(ctx: any) {
  const [orders, users]: number[] = await Promise.all([
    ctx.app.countRecords.async('orders'),
    ctx.app.countRecords.async('users'),
  ])
  return { orders, users }
}
```

What an async handler returns is used just like a returned object, and an error it throws (or a rejected promise)
fails the request like any other error. You can also `await` right in a template: see [EJS](ejs.md#await) and
[JSX](jsx.md#async-components).

### One `+page.js` per folder

A `+page.js` applies to every page in its folder. Use `ctx.route.name` to tell them apart:

```js
module.exports = {
  load(ctx) {
    if (ctx.route.name === 'details') {
      ctx.data.reviews = ctx.app.findRecordsByFilter('reviews', 'product = {:id}', '-created', 20, 0, { id: ctx.params.id })
    }
  },
}
```

### Methods without a handler

Every handler is optional. A page renders for any method, like PHP, so a template can handle a form on its own:

```ejs
<!-- pb_pages/contact.ejs, no +page.js -->
<%% if (ctx.req.method === 'POST') { %>
  <p>Thanks, <%%= ctx.req.body().name %>!</p>
<%% } else { %>
  <form method="post"><input name="name" /><button>Send</button></form>
<%% } %>
```

```tsx
// pb_pages/contact.tsx, no +page.ts
export default function Contact({ ctx }: any) {
  if (ctx.req.method === 'POST') return <p>Thanks, {ctx.req.body().name}!</p>
  return (
    <form method="post"><input name="name" /><button>Send</button></form>
  )
}
```

A `post` (or `put`, ...) handler in `+page.js` is the tidier place for form logic as a page grows, but you don't need
one. Only routes with nothing to render, [`+api.js`](api-routes.md) and a `+page.js` without a page, answer `405`
for a method they have no handler for.

## Example: a contact form

```js
// pb_pages/contact/+page.js
module.exports = {
  load(ctx) {
    return { departments: ctx.app.findAllRecords('departments') }
  },
  post(ctx) {
    const { email, message, department } = ctx.req.body()
    try {
      const record = new Record(ctx.app.findCollectionByNameOrId('messages'))
      record.set('email', email)
      record.set('message', message)
      record.set('department', department)
      ctx.app.save(record)
    } catch (e) {
      return { error: 'Failed to send message', values: { email, message, department } }
    }
    return ctx.res.redirect('/contact', { flash: 'Thanks, we got your message.' })
  },
}
```

```ts
// pb_pages/contact/+page.ts
type ContactForm = { email: string; message: string; department: string }

export function load(ctx: any) {
  return { departments: ctx.app.findAllRecords('departments') }
}

export function post(ctx: any) {
  const { email, message, department }: ContactForm = ctx.req.body()
  try {
    const record = new Record(ctx.app.findCollectionByNameOrId('messages'))
    record.set('email', email)
    record.set('message', message)
    record.set('department', department)
    ctx.app.save(record)
  } catch (e) {
    return { error: 'Failed to send message', values: { email, message, department } }
  }
  return ctx.res.redirect('/contact', { flash: 'Thanks, we got your message.' })
}
```

```ejs
<!-- pb_pages/contact/index.ejs -->
<%% const flash = ctx.flash() %>
<%% if (flash) { %><p><%%= flash %></p><%% } %>
<%% if (data.error) { %><mark><%%= data.error %></mark><%% } %>

<form method="post">
  <input name="email" type="email" value="<%%= data.values?.email %>" />
  <select name="department">
    <%% for (const d of data.departments) { %>
      <option value="<%%= d.id %>"><%%= d.getString('name') %></option>
    <%% } %>
  </select>
  <textarea name="message"><%%= data.values?.message %></textarea>
  <button type="submit">Send</button>
</form>
```

```tsx
// pb_pages/contact/index.tsx
export default function Contact({ ctx, data }: any) {
  const flash = ctx.flash()
  return (
    <>
      {flash && <p>{flash}</p>}
      {data.error && <mark>{data.error}</mark>}

      <form method="post">
        <input name="email" type="email" value={data.values?.email} />
        <select name="department">
          {data.departments.map((d: any) => (
            <option value={d.id}>{d.getString('name')}</option>
          ))}
        </select>
        <textarea name="message">{data.values?.message}</textarea>
        <button type="submit">Send</button>
      </form>
    </>
  )
}
```

## Pages without a template

A `+page.js` in a folder with no page file serves the folder's URL from JS. Use it for a page you'd rather build in
code, or for a URL that only acts and redirects:

```js
// pb_pages/logout/+page.js
module.exports = {
  post(ctx) {
    ctx.session.signOut()
    return ctx.res.redirect('/')
  },
}
```

```ts
// pb_pages/logout/+page.ts
export function post(ctx: any) {
  ctx.session.signOut()
  return ctx.res.redirect('/')
}
```

```js
// pb_pages/hello/+page.js
module.exports = {
  get(ctx) {
    return ctx.res.html(`<h1>Hello ${ctx.auth ? ctx.auth.getString('name') : 'there'}</h1>`)
  },
}
```

```ts
// pb_pages/hello/+page.ts
export function get(ctx: any) {
  return ctx.res.html(`<h1>Hello ${ctx.auth ? ctx.auth.getString('name') : 'there'}</h1>`)
}
```

- With no template to render, each handler must send a response; returning without one is a 500.
- Methods without a handler answer `405`, `GET` included: `/logout` above only takes `POST`.
- Folder middleware runs; layouts don't, so wrap HTML yourself (or render a partial with `ctx.include`).
- Errors show your `+error.ejs` page. For JSON responses and JSON errors, use [`+api.js`](api-routes.md) instead.

## Middleware or `+page.js`?

- Only the route's own folder's `+page.js` runs.
- Every `+middleware.js` from `pb_pages/` down to the route's folder runs, root first.

Use [middleware](middleware.md) for data or checks that a whole section of the site shares.
