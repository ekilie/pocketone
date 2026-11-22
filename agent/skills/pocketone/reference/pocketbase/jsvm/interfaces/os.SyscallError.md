Source: https://pocketbase.io/jsvm/interfaces/os.SyscallError.html

# Interface SyscallError

SyscallError records an error from a specific system call.

**Hierarchy**

- SyscallError

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

### syscall

```ts
syscall: string
```
