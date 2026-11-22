Source: https://pocketbase.io/jsvm/interfaces/core.FilesystemNewWriterEvent.html

# Interface FilesystemNewWriterEvent

**Hierarchy**

- [_swIkHtK](https://pocketbase.io/jsvm/types/core._swIkHtK.html)

  - FilesystemNewWriterEvent

## Methods

### next

```ts
next(): void
```

Next calls the next hook handler.

**Returns void**

## Properties

### app

```ts
app: App
```

### extra

```ts
extra: _TygojaDict
```

Extra holds any dynamic data that could be passed between event handlers.

### fileKey

```ts
fileKey: string
```

### Optional filesystem

```ts
filesystem?: System
```

### Optional options

```ts
options?: WriterOptions
```

### Optional writer

```ts
writer?: blob.Writer
```
