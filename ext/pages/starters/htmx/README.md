# htmx starter

A realtime demo with [htmx](https://htmx.org/) and the built-in `realtime` plugin. The chat form posts to `/fragments/chat`, which renders the message with a private partial and broadcasts it on the `chat` topic; two connections on the page receive it over PocketBase realtime (`pocketbase-htmx-ext-sse`). The counter button swaps in the HTML fragment from `/fragments/count`. Fragment routes live in `pb_pages/fragments/`, outside the `(site)` group, so no layout wraps them (`/api/*` belongs to PocketBase).

Templates are `.ets` (EJS whose tags hold TypeScript) and code files (`+config`, `+page`, `+api`, `+middleware`) are `.ts`; PocketPages strips the types when it loads them, so there is still no build step.

```bash
pocketone pages create myapp --starter htmx
cd myapp
pocketone superuser upsert you@example.com <password> --dir ./pb_data
pocketone serve --dir ./pb_data
```

Then open http://127.0.0.1:8090.
