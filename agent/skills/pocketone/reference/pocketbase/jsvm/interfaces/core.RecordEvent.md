Source: https://pocketbase.io/jsvm/interfaces/core.RecordEvent.html

# Interface RecordEvent

**Hierarchy**

- [_sGtwQJW](https://pocketbase.io/jsvm/types/core._sGtwQJW.html)

  - RecordEvent

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
