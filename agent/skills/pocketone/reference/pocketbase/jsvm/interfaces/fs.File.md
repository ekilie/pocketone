Source: https://pocketbase.io/jsvm/interfaces/fs.File.html

# Interface File

A File provides access to a single file. The File interface is the minimum implementation required of the file. Directory files should also implement [ReadDirFile]. A file may implement [io.ReaderAt] or [io.Seeker] as optimizations.

**Hierarchy**

- File

**Indexable**

```ts
[key: string]: any
```

## Methods

### close

```ts
close(): void
```

**Returns void**

### read

```ts
read(_arg0): number
```

**Parameters**

- `_arg0: string | number[]`

**Returns number**

### stat

```ts
stat(): fs.FileInfo
```

**Returns fs.FileInfo**
