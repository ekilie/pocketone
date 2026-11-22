Source: https://pocketbase.io/jsvm/interfaces/slog.LogValuer.html

# Interface LogValuer

A LogValuer is any Go value that can convert itself into a Value for logging.

This mechanism may be used to defer expensive operations until they are needed, or to expand a single value into a sequence of components.

**Hierarchy**

- LogValuer

**Indexable**

```ts
[key: string]: any
```

## Methods

### logValue

```ts
logValue(): Value
```

**Returns Value**
