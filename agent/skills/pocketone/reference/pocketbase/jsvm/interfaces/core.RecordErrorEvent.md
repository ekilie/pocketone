Source: https://pocketbase.io/jsvm/interfaces/core.RecordErrorEvent.html

# Interface RecordErrorEvent

**Hierarchy**

- [_sEBovep](https://pocketbase.io/jsvm/types/core._sEBovep.html)

  - RecordErrorEvent

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

### Optional record

```ts
record?: core.Record
```

### type

```ts
type: string
```

Could be any of the ModelEventType* constants, like:

- create
- update
- delete
- validate
