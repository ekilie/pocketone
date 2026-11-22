Source: https://pocketbase.io/jsvm/interfaces/fs.FileInfo.html

# Interface FileInfo

A FileInfo describes a file and is returned by [Stat].

**Hierarchy**

- FileInfo

  - [os](https://pocketbase.io/jsvm/modules/os.html).[FileInfo](https://pocketbase.io/jsvm/interfaces/os.FileInfo.html)

**Indexable**

```ts
[key: string]: any
```

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
