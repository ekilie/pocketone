Source: https://pocketbase.io/jsvm/interfaces/blob.ListObject.html

# Interface ListObject

ListObject represents a single blob returned from List.

**Hierarchy**

- ListObject

## Properties

### isDir

```ts
isDir: boolean
```

IsDir indicates that this result represents a "directory" in the hierarchical namespace, ending in ListOptions.Delimiter. Key can be passed as ListOptions.Prefix to list items in the "directory". Fields other than Key and IsDir will not be set if IsDir is true.

### key

```ts
key: string
```

Key is the key for this blob.

### md5

```ts
md5: string | number[]
```

MD5 is an MD5 hash of the blob contents or nil if not available.

### modTime

```ts
modTime: Time
```

ModTime is the time the blob was last modified.

### size

```ts
size: number
```

Size is the size of the blob's content in bytes.
