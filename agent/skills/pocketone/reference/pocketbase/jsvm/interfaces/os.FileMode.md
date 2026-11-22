Source: https://pocketbase.io/jsvm/interfaces/os.FileMode.html

# Interface FileMode

A FileMode represents a file's mode and permission bits. The bits have the same definition on all systems, so that information about files can be moved from one system to another portably. Not all bits apply to all systems. The only required bit is [ModeDir] for directories.

**Hierarchy**

- [fs](https://pocketbase.io/jsvm/modules/fs.html).[FileMode](https://pocketbase.io/jsvm/interfaces/fs.FileMode.html)

  - FileMode

## Methods

### isDir

```ts
isDir(): boolean
```

IsDir reports whether m describes a directory. That is, it tests for the [ModeDir] bit being set in m.

**Returns boolean**

### isRegular

```ts
isRegular(): boolean
```

IsRegular reports whether m describes a regular file. That is, it tests that no mode type bits are set.

**Returns boolean**

### perm

```ts
perm(): fs.FileMode
```

Perm returns the Unix permission bits in m (m & [ModePerm]).

**Returns fs.FileMode**

### string

```ts
string(): string
```

**Returns string**

### type

```ts
type(): fs.FileMode
```

Type returns type bits in m (m & [ModeType]).

**Returns fs.FileMode**
