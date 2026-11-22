Source: https://pocketbase.io/jsvm/interfaces/filesystem.NewWriterEvent.html

# Interface NewWriterEvent

**Hierarchy**

- [_sfTYkWS](https://pocketbase.io/jsvm/types/filesystem._sfTYkWS.html)

  - NewWriterEvent

## Methods

### next

```ts
next(): void
```

Next calls the next hook handler.

**Returns void**

## Properties

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
