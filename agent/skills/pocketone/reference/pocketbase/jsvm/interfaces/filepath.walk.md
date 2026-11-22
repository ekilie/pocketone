Source: https://pocketbase.io/jsvm/interfaces/filepath.walk.html

# Interface walk

**Hierarchy**

- walk

```ts
walk(root, fn): void
```

Walk walks the file tree rooted at root, calling fn for each file or directory in the tree, including root.

All errors that arise visiting files and directories are filtered by fn: see the [WalkFunc] documentation for details.

The files are walked in lexical order, which makes the output deterministic but requires Walk to read an entire directory into memory before proceeding to walk that directory.

Walk does not follow symbolic links.

Walk is less efficient than [WalkDir], introduced in Go 1.16, which avoids calling os.Lstat on every visited file or directory.

**Parameters**

- `root: string`
- `fn: WalkFunc`

**Returns void**
