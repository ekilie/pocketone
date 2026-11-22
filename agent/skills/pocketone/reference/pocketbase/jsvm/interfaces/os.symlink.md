Source: https://pocketbase.io/jsvm/interfaces/os.symlink.html

# Interface symlink

**Hierarchy**

- symlink

```ts
symlink(oldname, newname): void
```

Symlink creates newname as a symbolic link to oldname. On Windows, a symlink to a non-existent oldname creates a file symlink; if oldname is later created as a directory the symlink will not work. If there is an error, it will be of type *LinkError.

**Parameters**

- `oldname: string`
- `newname: string`

**Returns void**
