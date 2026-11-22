Source: https://pocketbase.io/jsvm/modules/_template.html

# Namespace $template

`$template` is a global helper to load and cache HTML templates on the fly.

The templates uses the standard Go [html/template](https://pkg.go.dev/html/template) and [text/template](https://pkg.go.dev/text/template) package syntax.

Example:

```js
const html = $template.loadFiles(
    "views/layout.html",
    "views/content.html",
).render({"name": "John"})
```
