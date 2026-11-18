---
title: Build a site
description: Serve server-rendered pages from a pb_pages folder next to your data, with PocketPages, built into every PocketOne app.
---

# Build a site

Put EJS or Markdown files in a `pb_pages` folder next to `pb_data`, and your app serves them as web pages. Each file
is a route, and page code can read and write your records directly. This is PocketPages, built into PocketOne.

It's on for every PocketOne app whenever a `pb_pages` folder exists. No folder, no pages. There's nothing to install
and no build step.

## Hello world

1. Make a folder for your app with a page in it:

   ```bash
   mkdir -p myapp/pb_pages && cd myapp
   echo '<%%= `Hello, world!` %>' > pb_pages/index.ejs
   ```

2. Create a superuser, then start the app:

   ```bash
   pocketone superuser upsert you@example.com <password> --dir ./pb_data
   pocketone serve --dir ./pb_data
   ```

3. Open http://127.0.0.1:8090. You see `Hello, world!`

Edit the file and reload the browser. Changes show up right away, with no restart.

To start from something bigger, `pocketone pages starters` lists the starter kits and
`pocketone pages create myapp --starter <kit>` makes a new app from one.

## What's covered

- **Files are routes:** `pb_pages/about.ejs` serves `/about`, and `pb_pages/blog/index.ejs` serves `/blog`.
- **Templates:** EJS, Markdown, JSX and TSX, with layouts, partials, middleware and JSON routes.
- **Leftovers go on:** a URL no page matches goes on to `pb_public`, then to PocketBase. URLs under `/api/` and `/_/`
  always belong to PocketBase.
- **Errors:** in dev mode you get detailed error pages. Otherwise visitors see a plain error page and the details go to
  the log.
- **Not with `--jsvm goja`:** pages run in PocketOne's own JS engine, so they're off when you serve with PocketBase's.

To send your pages to a running app, see [Deploy from your machine](/docs/features/deploy).

## Change it

To serve pages from another folder, pass `--pagesDir` to `pocketone serve`, or set `server.pages_dir` in
`pocketone.yaml` ([pocketone.yaml reference](/docs/config-reference)). Settings for the site itself, such as which
PocketPages plugins to load, go in `pb_pages/+config.js` ([+config.js](/docs/extensions/pages/config)).

## Learn more

The full PocketPages docs:

- [Installation](/docs/extensions/pages/installation) and [starter kits](/docs/extensions/pages/starter-kits)
- [Creating a page](/docs/extensions/pages/creating-a-page) and [routing](/docs/extensions/pages/routing)
- [Loading data](/docs/extensions/pages/loading-data)
- [Layouts and slots](/docs/extensions/pages/layouts)
- [Authentication](/docs/extensions/pages/authentication)
- [Going live](/docs/extensions/pages/deploying)

## How it works

- [Pages](/docs/extensions/pages/extension) serves the `pb_pages` folder and adds the `pages create` and
  `pages starters` commands. Page code runs in your app's JS engine.
- [PocketPages](/docs/extensions/pages) is the home of its docs, guides and plugins.
