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

## Run it

With a `pocketone` binary, from this folder:

```bash
pocketone superuser upsert admin@example.com password --dir ./pb_data
pocketone serve --dev --dir ./pb_data
```

PocketPages finds `pb_pages` next to `pb_data`. Open http://localhost:8090.
