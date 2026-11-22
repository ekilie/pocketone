Source: https://pocketbase.io/jsvm/interfaces/fs.DirEntry.html

# Interface DirEntry

A DirEntry is an entry read from a directory (using the [ReadDir] function or a [ReadDirFile]'s ReadDir method).

**Hierarchy**

- DirEntry

  - [os](https://pocketbase.io/jsvm/modules/os.html).[DirEntry](https://pocketbase.io/jsvm/interfaces/os.DirEntry.html)

**Indexable**

```ts
[key: string]: any
```

## Methods

### info

```ts
info(): fs.FileInfo
```

Info returns the FileInfo for the file or subdirectory described by the entry. The returned FileInfo may be from the time of the original directory read or from the time of the call to Info. If the file has been removed or renamed since the directory read, Info may return an error satisfying errors.Is(err, ErrNotExist). If the entry denotes a symbolic link, Info reports the information about the link itself, not the link's target.

**Returns fs.FileInfo**

### isDir

```ts
isDir(): boolean
```

IsDir reports whether the entry describes a directory.

**Returns boolean**

### name

```ts
name(): string
```

Name returns the name of the file (or subdirectory) described by the entry. This name is only the final element of the path (the base name), not the entire path. For example, Name would return "hello.go" not "home/gopher/hello.go".

**Returns string**

### type

```ts
type(): fs.FileMode
```

Type returns the type bits for the entry. The type bits are a subset of the usual FileMode bits, those returned by the FileMode.Type method.

**Returns fs.FileMode**
