---
title: New in PocketOne
description: Everything PocketOne adds to PocketBase, side by side with what PocketBase gives you. All built in and on by default.
---

# New in PocketOne

PocketOne does everything PocketBase does, and adds the features below. They're built in and on by default, whether
you run the `pocketone` binary yourself or a host runs your app. There's nothing to install.

## Modern JavaScript on the server

The two biggest differences are in the code you write. PocketBase can't do either of these.

**[Hooks with async, import and TypeScript](/docs/quickjs).** Your `pb_hooks` run on QuickJS with the same JS API you
already use, plus:

- `async` handlers and `await`. The app keeps serving other requests while a handler waits on the database or an HTTP
  call.
- ES modules: `import` and `export` between your files, and packages from `node_modules`.
- TypeScript: write `.pb.ts` files and they run as they are.
- Globals shared by every handler, and memory and time limits that stop a runaway script instead of the app.

```ts
// pb_hooks/orders.pb.ts
import { total } from "./lib/pricing.mjs"

routerAdd("GET", "/api/orders/{id}/total", async (e) => {
  const order = await e.app.findRecordById.async("orders", e.request.pathValue("id"))
  return e.json(200, { total: total(order) })
})
```

**[Pages in TSX, TypeScript or EJS](/docs/features/sites).** Put a `.tsx`, `.jsx`, `.ejs` or `.md` file in `pb_pages`
and it's a server-rendered page, with your records a function call away. Components `import` each other, and `+page.ts`
and `+api.ts` route files are TypeScript. esbuild is built into the binary: no Node, no `npm install`, no build step.

```ts
// pb_pages/products/[id]/+page.ts
export default {
  load(ctx: any) {
    ctx.data.product = ctx.app.findRecordById('products', ctx.params.id)
  },
}
```

```tsx
// pb_pages/products/[id]/index.tsx
import Price from './_private/Price'

export default function Product({ data }: any) {
  return (
    <article>
      <h1>{data.product.getString('name')}</h1>
      <Price cents={data.product.getInt('price')} />
    </article>
  )
}
```

## Everything new, side by side

| | PocketBase gives you | PocketOne adds |
|---|---|---|
| [QuickJS hooks](/docs/quickjs) | Hooks on goja: CommonJS, no `await`, a pool of separate runtimes | `async`/`await`, ES module `import`, TypeScript, shared globals and hard limits, on the same JS API |
| [PocketPages sites](/docs/features/sites) | Static files from `pb_public` | Server-rendered pages in TSX, JSX, TypeScript, EJS or Markdown, with no build step |
| [Point-in-time restore](/docs/features/restore) | Backups you schedule, restored whole | Every change recorded; restore to any second in the last 72 hours, and undo the restore |
| [Deploy and live sync](/docs/features/deploy) | Copy files to the server yourself, then restart | `pocketone deploy` uploads only what changed; `pocketone dev` syncs a folder both ways and streams logs |
| [Remote CLI](/docs/features/remote) | A CLI for the machine that holds `pb_data` | Admin commands against any app, from your terminal, a script or an agent |
| [API tokens and OAuth](/docs/features/tokens) | Auth tokens with the account's full rights | Named, scoped tokens you revoke one by one; your app is an OAuth server |
| [MCP for AI agents](/docs/features/mcp) | No MCP | `/mcp` on every app, with OAuth sign-in and your own tools from `pb_hooks` |
| [Bulk mail](/docs/features/bulk-mail) | Account mail: verification, password reset | Campaigns from the dashboard, with unsubscribes, suppression and pacing |
| [pocketone.yaml](/docs/pocketone-yaml) | Flags on every start | One config file, with every extension's options and a `.env` for secrets |

Each page shows the feature in the dashboard, from your terminal and from an agent. For how a feature works or how to
build on it, follow its **How it works** links to the [extension](/docs/extensions) pages.

## Also built in

Smaller additions you'll notice without a page of their own:

- **Mail you can see in dev mode:** with `serve --dev` and no SMTP settings, mail is caught and shown at
  `http://127.0.0.1:8025` instead of being sent ([Mail Trap](/docs/extensions/mailtrap)).
- **S3 redirects:** files kept in S3 download straight from the bucket through short-lived signed links
  ([S3 Redirect](/docs/extensions/s3redirect)).
- **Restylable sign-in pages:** copy the OAuth sign-in screens into your app with `pocketone template eject` and edit
  them ([Templates](/docs/extensions/templates)).
- **Settings and cache:** one settings screen and one shared memory budget for every feature
  ([Settings](/docs/extensions/settings), [Cache](/docs/extensions/cache)).

Coming from PocketBase? [What stays the same](/docs/pocketbase).
