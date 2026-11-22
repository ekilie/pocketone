Source: https://pocketbase.io/jsvm/interfaces/filepath.abs.html

# Interface abs

**Hierarchy**

- abs

```ts
abs(path): string
```

Abs returns an absolute representation of path. If the path is not absolute it will be joined with the current working directory to turn it into an absolute path. The absolute path name for a given file is not guaranteed to be unique. Abs calls [Clean] on the result.

**Parameters**

- `path: string`

**Returns string**
