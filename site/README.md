# pocketone.dev

The PocketOne website: what PocketOne is, how to install it, and its plugins. A PocketPages v1 app (`ext/pages`),
started from the pocketpages.dev site in [`../ext/pages/site`](../ext/pages/site).

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

- `plugins.json` lists every plugin: title, one-line summary and group. Every `ext/<name>` folder must be in it.
- Each `ext/<name>/README.md` becomes `/docs/plugins/<name>`, with the plugin's `pocketone.yaml` keys (from
  `pocketone config --json`, so it builds `examples/pocketone`) added to its `## Configure` section. Other `.md` files
  in the plugin folder become guides under it; `docs/plugins.md` becomes `/docs/writing-a-plugin`.
- It also writes `/docs/config-reference`: every `pocketone.yaml` section.
- Links between plugins and docs point at the site pages; links to source files point at GitHub.
- It also writes the plugin index (`/docs/plugins`) and the plugin part of the docs nav (between the markers in
  `pb_pages/(main)/_private/toc.md`).
- CI (`.github/workflows/site.yml`) runs `go run . --check` and fails when a page is out of date.

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
