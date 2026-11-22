Source: https://pocketbase.io/jsvm/functions/_filepath.walkDir.html

# Function walkDir

```ts
walkDir(root, fn): void
```

WalkDir walks the file tree rooted at root, calling fn for each file or directory in the tree, including root.

All errors that arise visiting files and directories are filtered by fn: see the [fs.WalkDirFunc] documentation for details.

The files are walked in lexical order, which makes the output deterministic but requires WalkDir to read an entire directory into memory before proceeding to walk that directory.

WalkDir does not follow symbolic links.

WalkDir calls fn with paths that use the separator character appropriate for the operating system. This is unlike [io/fs.WalkDir], which always uses slash separated paths.

**Parameters**

- `root: string`
- `fn: WalkDirFunc`

**Returns void**
