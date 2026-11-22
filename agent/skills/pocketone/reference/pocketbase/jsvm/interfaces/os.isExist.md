Source: https://pocketbase.io/jsvm/interfaces/os.isExist.html

# Interface isExist

**Hierarchy**

- isExist

```ts
isExist(err): boolean
```

IsExist returns a boolean indicating whether its argument is known to report that a file or directory already exists. It is satisfied by [ErrExist] as well as some syscall errors.

This function predates [errors.Is]. It only supports errors returned by the os package. New code should use errors.Is(err, fs.ErrExist).

**Parameters**

- `err: Error`

**Returns boolean**
