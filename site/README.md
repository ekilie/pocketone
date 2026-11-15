# pocketone.dev

The PocketOne website: what PocketOne is, how to install it, and its plugins. A PocketPages v1 app (`ext/core/pages`),
started from the pocketpages.dev site in [`../ext/core/pages/site`](../ext/core/pages/site).

## Layout

- `pb_pages/` is the app: the root layout, the home page in `(splash)/`, the docs in `(main)/docs/` and the blog in
  `(main)/blog/`.
- `pb_pages/(main)/_private/toc.md` is the docs navigation.
- `pb_pages/(main)/blog/`: every `.md` file there is a post (front matter: title, description, date, author, tags).
- `pb_pages/assets/site.css` is the whole stylesheet: hand-written, no build step.
- `design/`: the logo and mascot (the joey in the pocket). `mark.svg` is the source; `design/render.sh` renders
  `logo.html` to PNGs in `design/out/` and copies the mark and favicons into `pb_pages/`.

Core PocketBase topics are linked to pocketbase.io, never copied here.

## Plugin docs (`sync/`)

Plugin pages are copies of the READMEs in `ext/`, made by `sync/`. Don't edit them here; edit the README and run:

```bash
cd site/sync && go run .
```

- `plugins.json` lists every plugin: title, one-line summary and group. Every `ext/core/<name>` and `ext/community/<name>` folder must be in it.
- Each `ext/<tier>/<name>/README.md` becomes `/docs/plugins/<name>`, with the plugin's `pocketone.yaml` keys (from
  `pocketone config --json`, so it builds `examples/pocketone`) added to its `## Configure` section. Other `.md` files
  in the plugin folder become guides under it; `docs/plugins.md` becomes `/docs/writing-a-plugin`.
- It also writes `/docs/config-reference`: every `pocketone.yaml` section.
- Links between plugins and docs point at the site pages; links to source files point at GitHub.
- It also writes the plugin index (`/docs/plugins`) and the plugin part of the docs nav (between the markers in
  `pb_pages/(main)/_private/toc.md`).
- It also writes the search index, `pb_pages/assets/search-index.json`: every docs and blog page, hand-written ones
  included, one entry per H2-H4 section. **So after editing any page, run sync too.**
- CI (`.github/workflows/site.yml`) runs `go test .` and `go run . --check`, and fails when a page or the index is out
  of date.

## Search and comments

- **Search** (`pb_pages/assets/search.js`): the header button, `/` or Cmd/Ctrl-K opens a dialog that loads
  [MiniSearch](https://lucaong.github.io/minisearch/) and the index the first time it's used. No external service.
- **Comments** (`pb_pages/_private/docs-footer.js`, `pb_pages/assets/docs.js`): each docs page ends with a
  [giscus](https://giscus.app) box backed by GitHub Discussions (the Q&A category of pockethost/pocketone). A page's
  thread is found by an exact term, not its URL: `pocketone.dev/docs/plugins/<name>` for a plugin and all its guides,
  `pocketone.dev/<path>` for other pages. Front matter `discussion: <term>` picks another term (keep it when a page
  moves, so its thread follows); `discussion: false` turns comments off. The docs home and the plugin index have none.
  A thread is created on the first comment or reaction.
- Docs links swap only the page body (Datastar, scoped to the nav). A swap gets the page without its layouts, so
  `docs-footer.js` sends the footer (edit link, comments, tab title) along with it.

## Run it

With a `pocketone` binary, from this folder:

```bash
pocketone superuser upsert admin@example.com password --dir ./pb_data
pocketone serve --dev --dir ./pb_data
```

PocketPages finds `pb_pages` next to `pb_data`. Open http://localhost:8090.

## Deploy

Pushes to `main` deploy `site/pb_pages` to pocketone.dev once the sync check passes (`.github/workflows/site.yml`,
job `deploy`). It needs the `POCKETHOST_TOKEN` repository secret; without it the job skips. To deploy by hand:
`cd site && npx pockethost@latest deploy pocketone --no-node-modules`.
