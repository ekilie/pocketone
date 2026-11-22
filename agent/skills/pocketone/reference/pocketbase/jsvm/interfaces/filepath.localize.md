Source: https://pocketbase.io/jsvm/interfaces/filepath.localize.html

# Interface localize

**Hierarchy**

- localize

```ts
localize(path): string
```

Localize converts a slash-separated path into an operating system path. The input path must be a valid path as reported by [io/fs.ValidPath].

Localize returns an error if the path cannot be represented by the operating system. For example, the path a\b is rejected on Windows, on which \ is a separator character and cannot be part of a filename.

The path returned by Localize will always be local, as reported by IsLocal.

**Parameters**

- `path: string`

**Returns string**
