Source: https://pocketbase.io/jsvm/functions/_apis.static.html

# Function static

```ts
static(dirOrFS, indexFallback): ((e) => void)
```

Route handler to serve static directory content (html, js, css, etc.).

If a file resource is missing and indexFallback is true, the request will be forwarded to the base index.html (useful for SPA with pretty urls).

NB! Expects the route to have a "{path...}" wildcard parameter.

Special redirects:

- if "path" is a file that ends in index.html, it is redirected to its non-index.html version (eg. /test/index.html -> /test/)
- if "path" is a directory that has index.html, the index.html file is rendered, otherwise if missing - returns 404 or fallback to the root index.html if indexFallback is true

Example:

```js
// serves static files from the provided dir string path (it will be wrapped in $os.dirFS())
routerAdd("GET", "/{path...}", $apis.static("/path/to/public", false))

// serves static files from the explicit fs.FS value ($os.dirFS(), $os.openRoot().fs(), etc.)
routerAdd("GET", "/{path...}", $apis.static($os.dirFS("/path/to/public"), false))
```

**Parameters**

- `dirOrFS: string | FS`
- `indexFallback: boolean`

**Returns ((e) => void)**

```ts
(e): void
```

**Parameters**

- `e: RequestEvent`

**Returns void**
