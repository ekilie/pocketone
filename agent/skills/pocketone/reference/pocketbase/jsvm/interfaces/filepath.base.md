Source: https://pocketbase.io/jsvm/interfaces/filepath.base.html

# Interface base

**Hierarchy**

- base

```ts
base(path): string
```

Base returns the last element of path. Trailing path separators are removed before extracting the last element. If the path is empty, Base returns ".". If the path consists entirely of separators, Base returns a single separator.

**Parameters**

- `path: string`

**Returns string**
