Source: https://pocketbase.io/jsvm/interfaces/filesystem.File.html

# Interface File

File defines a single file [io.ReadSeekCloser] resource.

The file could be from a local path, multipart/form-data header, etc.

**Hierarchy**

- File

## Methods

### asMap

```ts
asMap(): _TygojaDict
```

AsMap implements [core.mapExtractor] and returns a value suitable to be used in an API rule expression.

**Returns _TygojaDict**

## Properties

### name

```ts
name: string
```

### originalName

```ts
originalName: string
```

### reader

```ts
reader: FileReader
```

### size

```ts
size: number
```
