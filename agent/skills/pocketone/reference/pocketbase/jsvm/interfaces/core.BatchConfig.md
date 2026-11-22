Source: https://pocketbase.io/jsvm/interfaces/core.BatchConfig.html

# Interface BatchConfig

**Hierarchy**

- BatchConfig

## Methods

### validate

```ts
validate(): void
```

Validate makes BatchConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### enabled

```ts
enabled: boolean
```

### maxBodySize

```ts
maxBodySize: number
```

MaxBodySize is the maximum allowed batch request body size in bytes.

If not set, fallbacks to max ~128MB.

### maxRequests

```ts
maxRequests: number
```

MaxRequests is the maximum allowed batch request to execute.

### timeout

```ts
timeout: number
```

Timeout is the max duration in seconds to wait before cancelling the batch transaction.
