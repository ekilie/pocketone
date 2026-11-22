# Datastar starter

A realtime chat with rooms and a live counter, built with [Datastar](https://data-star.dev/) and the Datastar add-on (`_private/datastar.js`, listed as `'datastar'`) and the built-in `realtime` plugin. Datastar's `@post` calls (the button, or Enter in the message box) hit `/fragments/chat` and `/fragments/count`, sending the page's signals as the JSON body that `ctx.datastar.signals()` reads. `/fragments/count` has a template: because the request carries `Datastar-Request: true`, the rendered page comes back as a `patchElements` event. `/fragments/chat` is a `+page.ts` with no template, so its `post()` builds the response itself. A chat message is also broadcast to every tab in the same room with `ctx.datastar.realtime.patchElements`, and `ctx.datastar.patchSignals` clears the input. Open two tabs to watch it update.

Templates are `.ets` (EJS whose tags hold TypeScript) and code files (`+config`, `+page`, `+api`, `+middleware`) are `.ts`; Pages strips the types when it loads them, so there is still no build step.

```bash
pocketone pages create myapp --starter datastar
cd myapp
pocketone superuser upsert you@example.com <password> --dir ./pb_data
pocketone serve --dir ./pb_data
```

Then open http://127.0.0.1:8090.
