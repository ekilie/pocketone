Source: https://pocketbase.io/jsvm/interfaces/os.fileStat.html

# Interface fileStat

A fileStat is the implementation of FileInfo returned by Stat and Lstat.

**Hierarchy**

- fileStat

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
mode(): os.FileMode
```

**Returns os.FileMode**

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
