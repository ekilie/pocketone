Source: https://pocketbase.io/jsvm/interfaces/core.RecordEnrichEvent.html

# Interface RecordEnrichEvent

**Hierarchy**

- [_sKVYWXG](https://pocketbase.io/jsvm/types/core._sKVYWXG.html)

  - RecordEnrichEvent

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

### Optional record

```ts
record?: core.Record
```

### Optional requestInfo

```ts
requestInfo?: RequestInfo
```
