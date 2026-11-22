Source: https://pocketbase.io/jsvm/interfaces/os.FileInfo.html

# Interface FileInfo

A FileInfo describes a file and is returned by [Stat] and [Lstat].

**Hierarchy**

- [fs](https://pocketbase.io/jsvm/modules/fs.html).[FileInfo](https://pocketbase.io/jsvm/interfaces/fs.FileInfo.html)

  - FileInfo

## Methods

### isDir

```ts
isDir(): boolean
```

**Returns boolean**

### modTime

```ts
modTime(): Time
```

**Returns Time**

### mode

```ts
mode(): fs.FileMode
```

**Returns fs.FileMode**

### name

```ts
name(): string
```

**Returns string**

### size

```ts
size(): number
```

**Returns number**

### sys

```ts
sys(): any
```

**Returns any**
