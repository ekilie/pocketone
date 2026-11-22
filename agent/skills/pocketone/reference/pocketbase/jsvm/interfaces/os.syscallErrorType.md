Source: https://pocketbase.io/jsvm/interfaces/os.syscallErrorType.html

# Interface syscallErrorType

An Errno is an unsigned number describing an error condition. It implements the error interface. The zero Errno is by convention a non-error, so code to convert from Errno to error should use:

```
  err = nil
  if errno != 0 {
    err = errno
  }
```

Errno values can be tested against error values using [errors.Is]. For example:

```
  _, _, err := syscall.Syscall(...)
  if errors.Is(err, fs.ErrNotExist) ...
```

**Hierarchy**

- [Errno](https://pocketbase.io/jsvm/interfaces/syscall.Errno.html)

  - syscallErrorType

## Methods

### error

```ts
error(): string
```

**Returns string**

### is

```ts
is(target): boolean
```

**Parameters**

- `target: Error`

**Returns boolean**

### temporary

```ts
temporary(): boolean
```

**Returns boolean**

### timeout

```ts
timeout(): boolean
```

**Returns boolean**
