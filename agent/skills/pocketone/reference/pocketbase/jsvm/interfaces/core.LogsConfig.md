Source: https://pocketbase.io/jsvm/interfaces/core.LogsConfig.html

# Interface LogsConfig

**Hierarchy**

- LogsConfig

## Methods

### validate

```ts
validate(): void
```

Validate makes LogsConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### logAuthId

```ts
logAuthId: boolean
```

### logIP

```ts
logIP: boolean
```

### maxDataSize

```ts
maxDataSize: number
```

MaxDataSize specifies the maximum allowed serialized log data size before it gets truncated (see [Log.DBExport]).

If zero, fallbacks to ~16kb by default.

### maxDays

```ts
maxDays: number
```

### minLevel

```ts
minLevel: number
```
