Source: https://pocketbase.io/jsvm/interfaces/core.CollectionErrorEvent.html

# Interface CollectionErrorEvent

**Hierarchy**

- [_sxLWbWO](https://pocketbase.io/jsvm/types/core._sxLWbWO.html)

  - CollectionErrorEvent

## Methods

### next

```ts
next(): void
```

Next calls the next hook handler.

**Returns void**

### tags

```ts
tags(): string[]
```

**Returns string[]**

### Todo

consider storing the original collection name and use that as a tag to avoid the ambiguity when the collection is being modified (#7613); for new collection also maybe return empty tags?

## Properties

### app

```ts
app: App
```

### Optional collection

```ts
collection?: core.Collection
```

### context

```ts
context: context.Context
```

### error

```ts
error: Error
```

### extra

```ts
extra: _TygojaDict
```

Extra holds any dynamic data that could be passed between event handlers.

### type

```ts
type: string
```

Could be any of the ModelEventType* constants, like:

- create
- update
- delete
- validate
