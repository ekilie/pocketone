# DaisyUI starter

A layout with a responsive navbar and drawer, built with [Tailwind CSS](https://tailwindcss.com/) and [DaisyUI](https://daisyui.com/) (cupcake theme). The compiled `pb_pages/app.css` is included and linked through `asset()`, so the app runs as-is; npm is only for rebuilding the CSS.

Templates are `.ets` (EJS whose tags hold TypeScript) and code files (`+config`, `+page`, `+api`, `+middleware`) are `.ts`; PocketPages strips the types when it loads them, so there is still no build step.

```bash
pocketone pages create myapp --starter daisyui
cd myapp
pocketone superuser upsert you@example.com <password> --dir ./pb_data
pocketone serve --dir ./pb_data
```

Then open http://127.0.0.1:8090.

To change styles, rebuild the CSS while you edit (Tailwind scans `pb_pages/**/*.{ets,md}`):

```bash
npm i
npm run dev:css
```
