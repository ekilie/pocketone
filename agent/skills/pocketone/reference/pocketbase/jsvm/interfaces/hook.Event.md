Source: https://pocketbase.io/jsvm/interfaces/hook.Event.html

# Interface Event

Event implements [Resolver] and it is intended to be used as a base Hook event that you can embed in your custom typed event structs.

Example:

```
  type CustomEvent struct {
    hook.Event

    SomeField int
  }
```

**Hierarchy**

- Event

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
