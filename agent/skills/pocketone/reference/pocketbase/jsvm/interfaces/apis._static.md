Source: https://pocketbase.io/jsvm/interfaces/apis._static.html

# Interface _static

**Hierarchy**

- _static

```ts
_static(fsys, indexFallback): ((_arg0) => void)
```

Static is a handler function to serve static directory content from fsys.

If a file resource is missing and indexFallback is true, the request will be forwarded to the base index.html (useful for SPA with pretty urls).

NB! Expects the route to have a "{path...}" wildcard parameter.

Special redirects:

```
  - if "path" is a file that ends in index.html, it is redirected to its non-index.html version (eg. /test/index.html -> /test/)
  - if "path" is a directory that has index.html, the index.html file is rendered,
    otherwise if missing - returns 404 or fallback to the root index.html if indexFallback is true
```

Example:

```
 fsys := os.DirFS("./pb_public")
 router.GET("/files/{path...}", apis.Static(fsys, false))
```

**Parameters**

- `fsys: FS`
- `indexFallback: boolean`

**Returns ((_arg0) => void)**

```ts
(_arg0): void
```

Static is a handler function to serve static directory content from fsys.

If a file resource is missing and indexFallback is true, the request will be forwarded to the base index.html (useful for SPA with pretty urls).

NB! Expects the route to have a "{path...}" wildcard parameter.

Special redirects:

```
  - if "path" is a file that ends in index.html, it is redirected to its non-index.html version (eg. /test/index.html -> /test/)
  - if "path" is a directory that has index.html, the index.html file is rendered,
    otherwise if missing - returns 404 or fallback to the root index.html if indexFallback is true
```

Example:

```
 fsys := os.DirFS("./pb_public")
 router.GET("/files/{path...}", apis.Static(fsys, false))
```

**Parameters**

- `_arg0: RequestEvent`

**Returns void**
