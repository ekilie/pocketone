Source: https://pocketbase.io/jsvm/interfaces/os.isNotExist.html

# Interface isNotExist

**Hierarchy**

- isNotExist

```ts
isNotExist(err): boolean
```

IsNotExist returns a boolean indicating whether its argument is known to report that a file or directory does not exist. It is satisfied by [ErrNotExist] as well as some syscall errors.

This function predates [errors.Is]. It only supports errors returned by the os package. New code should use errors.Is(err, fs.ErrNotExist).

**Parameters**

- `err: Error`

**Returns boolean**
