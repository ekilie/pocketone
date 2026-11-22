Source: https://pocketbase.io/jsvm/interfaces/fs.PathError.html

# Interface PathError

PathError records an error and the operation and file path that caused it.

**Hierarchy**

- PathError

  - [os](https://pocketbase.io/jsvm/modules/os.html).[PathError](https://pocketbase.io/jsvm/interfaces/os.PathError.html)

## Methods

### error

```ts
error(): string
```

**Returns string**

### timeout

```ts
timeout(): boolean
```

Timeout reports whether this error represents a timeout.

**Returns boolean**

### unwrap

```ts
unwrap(): void
```

**Returns void**

## Properties

### err

```ts
err: Error
```

### op

```ts
op: string
```

### path

```ts
path: string
```
