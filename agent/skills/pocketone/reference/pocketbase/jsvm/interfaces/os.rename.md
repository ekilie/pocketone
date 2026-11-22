Source: https://pocketbase.io/jsvm/interfaces/os.rename.html

# Interface rename

**Hierarchy**

- rename

```ts
rename(oldpath, newpath): void
```

Rename renames (moves) oldpath to newpath. If newpath already exists and is not a directory, Rename replaces it. If newpath already exists and is a directory, Rename returns an error. OS-specific restrictions may apply when oldpath and newpath are in different directories. Even within the same directory, on non-Unix platforms Rename is not an atomic operation. If there is an error, it will be of type *LinkError.

**Parameters**

- `oldpath: string`
- `newpath: string`

**Returns void**
