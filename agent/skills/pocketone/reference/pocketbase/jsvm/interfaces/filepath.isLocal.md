Source: https://pocketbase.io/jsvm/interfaces/filepath.isLocal.html

# Interface isLocal

**Hierarchy**

- isLocal

```ts
isLocal(path): boolean
```

IsLocal reports whether path, using lexical analysis only, has all of these properties:

```
  - is within the subtree rooted at the directory in which path is evaluated
  - is not an absolute path
  - is not empty
  - on Windows, is not a reserved name such as "NUL"
```

If IsLocal(path) returns true, then Join(base, path) will always produce a path contained within base and Clean(path) will always produce an unrooted path with no ".." path elements.

IsLocal is a purely lexical operation. In particular, it does not account for the effect of any symbolic links that may exist in the filesystem.

**Parameters**

- `path: string`

**Returns boolean**
