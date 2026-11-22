Source: https://pocketbase.io/jsvm/interfaces/os.copyFS.html

# Interface copyFS

**Hierarchy**

- copyFS

```ts
copyFS(dir, fsys): void
```

CopyFS copies the file system fsys into the directory dir, creating dir if necessary.

Files are created with mode 0o666 plus any execute permissions from the source, and directories are created with mode 0o777 (before umask).

CopyFS will not overwrite existing files. If a file name in fsys already exists in the destination, CopyFS will return an error such that errors.Is(err, fs.ErrExist) will be true.

Symbolic links in dir are followed.

New files added to fsys (including if dir is a subdirectory of fsys) while CopyFS is running are not guaranteed to be copied.

Copying stops at and returns the first error encountered.

**Parameters**

- `dir: string`
- `fsys: FS`

**Returns void**
