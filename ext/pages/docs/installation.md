---
title: Installation
description: Run PocketPages v1 on PocketOne with the pages extension, using a pb_pages folder.
---

# Installation

PocketPages v1 is built into PocketOne as the `pages` extension (`ext/pages`). There is nothing to install from npm: the standard `pocketone` binary includes it.

<%- include('quickstart') %>

`pocketone pages create myapp` creates a new app. Add `--starter <kit>` to start from a [starter kit](starter-kits.md).

_Note: `--dir` tells `pocketone` where `pb_data` is. PocketPages looks for `pb_pages` next to it. Create the superuser before the first `serve`; otherwise PocketBase opens its superuser installer in your browser._

Browse to `http://localhost:8090` and you'll see the starter's home page. The default `minimal` kit is one splash page:

<%- include('browser', { content: `<h1>PocketPages</h1>` }) %>

To start editing, open

```
myapp/pb_pages/index.ets
```

Changes appear on the next refresh. The app doesn't restart.

## Where pages live

Pages live in their own folder next to PocketBase's others:

```
myapp/
  pb_data/  pb_hooks/  pb_migrations/  pb_public/
  pb_pages/
```

- PocketPages finds the folder at `pb_pages` next to `pb_data`, or wherever `--pagesDir` points.
- If the folder doesn't exist, the extension does nothing for that app.
- Under a multitenant host, every instance has its own `pb_pages`.

## How does it work?

PocketPages is a multi-page app server for PocketOne. It listens for requests and renders pages using file-based routing rules, like old-school PHP. URLs under `/api/` and `/_/` always belong to PocketBase. A URL nothing in `pb_pages` matches goes on to `pb_public`, then to PocketBase.
