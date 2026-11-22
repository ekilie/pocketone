Source: https://pocketbase.io/jsvm/interfaces/os.mkdirAll.html

# Interface mkdirAll

**Hierarchy**

- mkdirAll

```ts
mkdirAll(path, perm): void
```

MkdirAll creates a directory named path, along with any necessary parents, and returns nil, or else returns an error. The permission bits perm (before umask) are used for all directories that MkdirAll creates. If path is already a directory, MkdirAll does nothing and returns nil.

**Parameters**

- `path: string`
- `perm: os.FileMode`

**Returns void**
