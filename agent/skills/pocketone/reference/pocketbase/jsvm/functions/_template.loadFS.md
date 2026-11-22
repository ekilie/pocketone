Source: https://pocketbase.io/jsvm/functions/_template.loadFS.html

# Function loadFS

```ts
loadFS(fsys, ...globPatterns): Renderer
```

LoadFS caches (if not already) the specified fs and globPatterns pair as single template and returns a ready to use Renderer instance.

There must be at least 1 file matching the provided globPattern(s) (note that most file names serves as glob patterns matching themselves).

**Parameters**

- `fsys: FS`
- `Rest ...globPatterns: string[]`

**Returns Renderer**
