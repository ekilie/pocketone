Source: https://pocketbase.io/jsvm/interfaces/filepath.glob.html

# Interface glob

**Hierarchy**

- glob

```ts
glob(pattern): string[]
```

Glob returns the names of all files matching pattern or nil if there is no matching file. The syntax of patterns is the same as in [Match]. The pattern may describe hierarchical names such as /usr/*/bin/ed (assuming the [Separator] is '/').

Glob ignores file system errors such as I/O errors reading directories. The only possible returned error is [ErrBadPattern], when pattern is malformed.

**Parameters**

- `pattern: string`

**Returns string[]**
