Source: https://pocketbase.io/jsvm/interfaces/apis.mustSubFS.html

# Interface mustSubFS

**Hierarchy**

- mustSubFS

```ts
mustSubFS(fsys, dir): FS
```

MustSubFS returns an [fs.FS] corresponding to the subtree rooted at fsys's dir.

This is similar to [fs.Sub] but panics on failure.

**Parameters**

- `fsys: FS`
- `dir: string`

**Returns FS**
