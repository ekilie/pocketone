---
title: Overview of PocketPages
description: PocketPages is server-rendered multi-page apps for PocketOne, from a pb_pages folder of EJS and Markdown files.
---

# Overview of PocketPages

PocketPages turns a folder of files into a server-rendered website, inside PocketOne. It is built in as the `pages` extension (`ext/pages`): no npm package, no boot file, no build step.

Use PocketPages to create slim and modern web apps:

- Start small and iterate
- SEO-first principles
- No build step
- Fast server-side rendering

## Hello world in just one file

```ejs
<!-- pb_pages/index.ejs -->
<%%= `Hello, world!` %>
```

```tsx
// pb_pages/index.tsx
export default () => <>{`Hello, world!`}</>
```

<%- include('browser', { content: `Hello, world!` }) %>

Inside `<%%` and `%>` you write plain JavaScript. Every page gets a `ctx` object with the request, the response, your data, and the app itself (`ctx.app`), so you can read and write records directly.

## Retro file-based routing

Files become URLs, and `+layout.ejs` files wrap the pages below them:

```ejs
<!-- pb_pages/+layout.ejs -->
<html>
  <body style="background-color: #b5dcb5">
    <%%- slot() %>
  </body>
</html>
```

```tsx
// pb_pages/+layout.tsx
export default ({ children }: any) => (
  <html>
    <body style="background-color: #b5dcb5">{children}</body>
  </html>
)
```

<%- include('browser', { bodyTags: `style="background-color: #b5dcb5"`, content: `Hello, world!` }) %>

## Coming from the npm package?

PocketPages v1 is a clean API. Apps written for PocketPages 0.x (the npm package) don't run unchanged. See [Moving from PocketPages 0.x](moving-from-0x.md).
