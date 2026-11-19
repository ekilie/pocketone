---
title: EJS
description: The EJS tags PocketPages supports, escaping rules, and what's in scope inside a template.
---

# EJS

EJS is always on. There is nothing to install or configure.

## Tags

| Tag | Meaning |
|---|---|
| `<%% code %>` | Run JS, no output |
| `<%%= expr %>` | Output, HTML-escaped |
| `<%%- expr %>` | Output, raw |
| `<%%# comment %>` | Nothing |
| `<%%%` | A literal `<%%` |
| `-%>` | Also remove the newline right after the tag |
| `<%%_` / `_%>` | Also remove whitespace before / after the tag on its line |
| `<script server<%- '>' %> code </script>` | Same as `<%% code %>` |

## Escaping

`<%%= %>` replaces `&` `<` `>` `"` `'` with `&amp;` `&lt;` `&gt;` `&#34;` `&#39;`. `null` and `undefined` output as an empty string.

```ejs
<p><%%= '<b>"x"</b>' %></p>   <!-- <p>&lt;b&gt;&#34;x&#34;&lt;/b&gt;</p> -->
<p><%%- '<b>raw</b>' %></p>    <!-- <p><b>raw</b></p> -->
<p><%%= null %></p>            <!-- <p></p> -->
```

Use `<%%-` only for HTML you trust, such as the output of `include()` or `slot()`.

## In scope

Inside a template you can use:

| Name | |
|---|---|
| `ctx` | The [request context](ctx/index.md) |
| `data` | `ctx.data` |
| `params` | `ctx.params` |
| `include` | Render a [partial](partials.md) |
| `slot` | Define or read a [slot](layouts.md) |
| `asset` | `ctx.asset` |
| `meta` | `ctx.meta` |
| `require` | Load a module: a path, or a name looked up in `_private/` then `node_modules` (see [JavaScript Runtime](jsvm.md)) |

## Output

A template returns its text. There is no `echo()`; anything you want in the output goes through a tag.

```ejs
<ul>
<%% for (const n of [1, 2]) { -%>
  <li><%%= n %></li>
<%% } -%>
</ul>
```

## await

A template can `await`. Any tag that uses `await` turns the template into an async one, and PocketPages waits for it
before it sends the page:

```ejs
<%% const orders = await ctx.app.countRecords.async('orders') %>
<p><%%= orders %> orders</p>
```

- Including a partial that uses `await` gives you a promise. Print it with `<%%- await include('stats') %>`.
- Printing a promise without `await` is an error, so you won't get `[object Promise]` in your page.
- A slot whose content awaits takes an async function: `<%% await slot('title', async () => { %>...<%% }) %>`.
- A template without `await` runs as a plain function, so sync templates cost nothing extra.

## Caching

A template is compiled once per content hash and reused, so an edited file is recompiled on the next request.
