---
title: Starters
description: The Pages starter kits - minimal, mvp, auth, daisyui, daisyui-docs, htmx, datastar and tsx - what each shows, how it's built and how to start an app from one.
---

# Starters

Each starter kit is an ordinary `pb_pages` folder written against Pages, in TypeScript with
[ETS templates](/docs/extensions/pages/ets) (the **tsx** kit uses [components](/docs/extensions/pages/jsx) instead), so
everything in the [Pages docs](/docs/extensions/pages) applies to it. Start an app from one, then run it:

```bash
pocketone pages create myapp --starter <kit>
cd myapp
pocketone superuser upsert you@example.com <password> --dir ./pb_data
pocketone serve --dir ./pb_data
```

`pocketone pages create myapp` with no `--starter` uses **minimal**, and `pocketone pages starters` lists the kits. The
folder must not exist yet. Each kit's source is in
[`ext/pages/starters`](https://github.com/pocketonehq/pocketone/tree/main/ext/pages/starters).

## minimal

A single `index.ets` page: a Tailwind (CDN) splash screen with no layout, no config and no build step. The smallest
possible PocketOne app. Live demo: [pocketpages-minimal.pockethost.io](https://pocketpages-minimal.pockethost.io).

```
pb_pages/
  index.ets
```

## mvp

One page styled with [MVP.css](https://andybrewer.github.io/mvp/): classless CSS that makes plain semantic HTML look
good, for a minimal, responsive, mobile-friendly site with no build step. Live demo:
[pocketpages-mvp.pockethost.io](https://pocketpages-mvp.pockethost.io).

```
pb_pages/
  index.ets
```

## auth

A multi-page app with the built-in `auth` plugin: register, password, anonymous, OTP and OAuth2 login, logout, email
verification (with a resend page), password reset by email, email change with confirmation, and a protected `/account`
section. Sessions are a `pb_auth` cookie managed through `ctx.session.*`; the signed-in user is `ctx.auth`. Its
migrations point PocketBase's mail at a local maildev and its email templates at this app's `/auth/confirm/{TOKEN}/...`
pages. Live demo: [pocketpages-auth.pockethost.io](https://pocketpages-auth.pockethost.io).

```
pb_migrations/                   the users collection's auth settings and email templates
pb_pages/
  +config.ts                     turns on the auth plugin
  +layout.ets
  auth/login/                    +page.ts signs in; _private/ holds one form per method (password, otp, oauth2, ...)
  auth/forgot/                   asks for a password reset email
  auth/confirm/[token]/          the pages the emails link to: verification, password reset, email change
  account/+middleware.ts         sends anyone not signed in to the login page
  account/                       the signed-in user's page, and change-email
```

## daisyui

A layout with a responsive navbar and drawer, built with [Tailwind CSS](https://tailwindcss.com/) and
[DaisyUI](https://daisyui.com/) (cupcake theme). The compiled `pb_pages/app.css` is included and linked through
`asset()`, so the app runs as-is; npm is only for rebuilding the CSS. Live demo:
[pocketpages-daisyui.pockethost.io](https://pocketpages-daisyui.pockethost.io).

```
pb_pages/
  +layout.ets                    the navbar and drawer around every page
  index.ets
  app.css                        compiled Tailwind and DaisyUI
```

## daisyui-docs

A docs site built from Markdown pages, styled with Tailwind CSS and DaisyUI. It shows layout groups (`(splash)` for the
home page, `(main)` for the docs), nested layouts, and a Markdown partial (`docs/_private/toc.md`) as the table of
contents. Add a page by dropping a `.md` file in `pb_pages/(main)/docs/`. Live demo:
[pocketpages-daisyui-docs.pockethost.io](https://pocketpages-daisyui-docs.pockethost.io).

```
pb_pages/
  +layout.ets
  (splash)/index.ets             the home page, in its own layout
  (main)/+layout.ets
  (main)/docs/+layout.ets        the docs layout, with the table of contents
  (main)/docs/_private/toc.md    the table of contents
  (main)/docs/overview.md        a docs page: add more beside it
```

## htmx

A realtime demo with [htmx](https://htmx.org/) and the built-in `realtime` plugin. The chat form posts to
`/fragments/chat`, which renders the message with a private partial and broadcasts it on the `chat` topic; open
connections receive it over PocketBase realtime. The counter button swaps in the HTML fragment from `/fragments/count`.
Fragment routes live in `pb_pages/fragments/`, outside the `(site)` group, so no layout wraps them. Live demo:
[pocketpages-htmx.pockethost.io](https://pocketpages-htmx.pockethost.io).

```
pb_pages/
  +config.ts                     turns on the realtime plugin
  (site)/+layout.ets, index.ets  the page, in the site layout
  _private/                      the chat message and counter partials
  fragments/chat/                renders a message and broadcasts it
  fragments/count/               the counter fragment
```

## datastar

A realtime chat with rooms and a live counter, built with [Datastar](https://data-star.dev/), the Datastar add-on
(`_private/datastar.js`) and the built-in `realtime` plugin. Datastar's `@post` calls hit `/fragments/chat` and
`/fragments/count`, sending the page's signals as the JSON body that `ctx.datastar.signals()` reads. A chat message is
also broadcast to every tab in the same room with `ctx.datastar.realtime.patchElements`. Open two tabs to watch it
update. Live demo: [pocketpages-datastar.pockethost.io](https://pocketpages-datastar.pockethost.io).

```
pb_pages/
  +config.ts                     the datastar add-on and the realtime plugin
  (site)/index.ets               the lobby
  (site)/[roomName]/index.ets    a chat room
  _private/                      the chat, message and counter partials, and datastar.js
  fragments/chat/+page.ts        posts a message and patches every tab in the room
  fragments/count/               the counter, returned as a patchElements event
```

## tsx

Server-rendered components: a `+layout.tsx`, an `index.tsx` page built from a `_private/Feature.tsx` component, and a
`+page.ts` that loads the page's data. TSX renders straight to HTML on the server (no React, no client runtime, no
hydration), and Pages strips the types when it loads each file, so there is no build step. Pick one template
style per app: TSX here, or `.ets` in the other kits.

```
pb_pages/
  +layout.tsx
  +page.ts                       loads the page's data
  index.tsx
  _private/Feature.tsx           a component
```
