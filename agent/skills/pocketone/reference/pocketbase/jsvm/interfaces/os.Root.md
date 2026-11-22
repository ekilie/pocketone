Source: https://pocketbase.io/jsvm/interfaces/os.Root.html

# Interface Root

Root may be used to only access files within a single directory tree.

Methods on Root can only access files and directories beneath a root directory. If any component of a file name passed to a method of Root references a location outside the root, the method returns an error. File names may reference the directory itself (.).

Methods on Root will follow symbolic links, but symbolic links may not reference a location outside the root. Symbolic links must not be absolute.

Methods on Root do not prohibit traversal of filesystem boundaries, Linux bind mounts, /proc special files, or access to Unix device files.

Methods on Root are safe to be used from multiple goroutines simultaneously.

On most platforms, creating a Root opens a file descriptor or handle referencing the directory. If the directory is moved, methods on Root reference the original directory in its new location.

Root's behavior differs on some platforms:

```
  - When GOOS=windows, file names may not reference Windows reserved device names
    such as NUL and COM1.
  - On Unix, [Root.Chmod], [Root.Chown], and [Root.Chtimes] are vulnerable to a race condition.
    If the target of the operation is changed from a regular file to a symlink
    while the operation is in progress, the operation may be performed on the link
    rather than the link target.
  - When GOOS=js, Root is vulnerable to TOCTOU (time-of-check-time-of-use)
    attacks in symlink validation, and cannot ensure that operations will not
    escape the root.
  - When GOOS=plan9 or GOOS=js, Root does not track directories across renames.
    On these platforms, a Root references a directory name, not a file descriptor.
  - WASI preview 1 (GOOS=wasip1) does not support [Root.Chmod].
```

**Hierarchy**

- Root

  - [rootFS](https://pocketbase.io/jsvm/interfaces/os.rootFS.html)

## Methods

### chmod

```ts
chmod(name, mode): void
```

Chmod changes the mode of the named file in the root to mode. See [Chmod] for more details.

**Parameters**

- `name: string`
- `mode: os.FileMode`

**Returns void**

### chown

```ts
chown(name, uid, gid): void
```

Chown changes the numeric uid and gid of the named file in the root. See [Chown] for more details.

**Parameters**

- `name: string`
- `uid: number`
- `gid: number`

**Returns void**

### chtimes

```ts
chtimes(name, atime, mtime): void
```

Chtimes changes the access and modification times of the named file in the root. See [Chtimes] for more details.

**Parameters**

- `name: string`
- `atime: Time`
- `mtime: Time`

**Returns void**

### close

```ts
close(): void
```

Close closes the Root. After Close is called, methods on Root return errors.

**Returns void**

### create

```ts
create(name): os.File
```

Create creates or truncates the named file in the root. See [Create] for more details.

**Parameters**

- `name: string`

**Returns os.File**

### fs

```ts
fs(): FS
```

FS returns a file system (an fs.FS) for the tree of files in the root.

The result implements [io/fs.StatFS], [io/fs.ReadFileFS], [io/fs.ReadDirFS], and [io/fs.ReadLinkFS].

**Returns FS**

### lchown

```ts
lchown(name, uid, gid): void
```

Lchown changes the numeric uid and gid of the named file in the root. See [Lchown] for more details.

**Parameters**

- `name: string`
- `uid: number`
- `gid: number`

**Returns void**

### link

```ts
link(oldname, newname): void
```

Link creates newname as a hard link to the oldname file. Both paths are relative to the root. See [Link] for more details.

If oldname is a symbolic link, Link creates new link to oldname and not its target. This behavior may differ from that of [Link] on some platforms.

When GOOS=js, Link returns an error if oldname is a symbolic link.

**Parameters**

- `oldname: string`
- `newname: string`

**Returns void**

### lstat

```ts
lstat(name): os.FileInfo
```

Lstat returns a [FileInfo] describing the named file in the root. If the file is a symbolic link, the returned FileInfo describes the symbolic link. See [Lstat] for more details.

**Parameters**

- `name: string`

**Returns os.FileInfo**

### mkdir

```ts
mkdir(name, perm): void
```

Mkdir creates a new directory in the root with the specified name and permission bits (before umask). See [Mkdir] for more details.

If perm contains bits other than the nine least-significant bits (0o777), Mkdir returns an error.

**Parameters**

- `name: string`
- `perm: os.FileMode`

**Returns void**

### mkdirAll

```ts
mkdirAll(name, perm): void
```

MkdirAll creates a new directory in the root, along with any necessary parents. See [MkdirAll] for more details.

If perm contains bits other than the nine least-significant bits (0o777), MkdirAll returns an error.

**Parameters**

- `name: string`
- `perm: os.FileMode`

**Returns void**

### name

```ts
name(): string
```

Name returns the name of the directory presented to OpenRoot.

It is safe to call Name after [Close].

**Returns string**

### open

```ts
open(name): os.File
```

Open opens the named file in the root for reading. See [Open] for more details.

**Parameters**

- `name: string`

**Returns os.File**

### openFile

```ts
openFile(name, flag, perm): os.File
```

OpenFile opens the named file in the root. See [OpenFile] for more details.

If perm contains bits other than the nine least-significant bits (0o777), OpenFile returns an error.

**Parameters**

- `name: string`
- `flag: number`
- `perm: os.FileMode`

**Returns os.File**

### openRoot

```ts
openRoot(name): Root
```

OpenRoot opens the named directory in the root. If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`

**Returns Root**

### readFile

```ts
readFile(name): string | number[]
```

ReadFile reads the named file in the root and returns its contents. See [ReadFile] for more details.

**Parameters**

- `name: string`

**Returns string | number[]**

### readlink

```ts
readlink(name): string
```

Readlink returns the destination of the named symbolic link in the root. See [Readlink] for more details.

**Parameters**

- `name: string`

**Returns string**

### remove

```ts
remove(name): void
```

Remove removes the named file or (empty) directory in the root. See [Remove] for more details.

**Parameters**

- `name: string`

**Returns void**

### removeAll

```ts
removeAll(name): void
```

RemoveAll removes the named file or directory and any children that it contains. See [RemoveAll] for more details.

**Parameters**

- `name: string`

**Returns void**

### rename

```ts
rename(oldname, newname): void
```

Rename renames (moves) oldname to newname. Both paths are relative to the root. See [Rename] for more details.

**Parameters**

- `oldname: string`
- `newname: string`

**Returns void**

### stat

```ts
stat(name): os.FileInfo
```

Stat returns a [FileInfo] describing the named file in the root. See [Stat] for more details.

**Parameters**

- `name: string`

**Returns os.FileInfo**

### symlink

```ts
symlink(oldname, newname): void
```

Symlink creates newname as a symbolic link to oldname. See [Symlink] for more details.

Symlink does not validate oldname, which may reference a location outside the root.

On Windows, a directory link is created if oldname references a directory within the root. Otherwise a file link is created.

**Parameters**

- `oldname: string`
- `newname: string`

**Returns void**

### writeFile

```ts
writeFile(name, data, perm): void
```

WriteFile writes data to the named file in the root, creating it if necessary. See [WriteFile] for more details.

**Parameters**

- `name: string`
- `data: string | number[]`
- `perm: os.FileMode`

**Returns void**
