Source: https://pocketbase.io/jsvm/interfaces/os.isPermission.html

# Interface isPermission

**Hierarchy**

- isPermission

```ts
isPermission(err): boolean
```

IsPermission returns a boolean indicating whether its argument is known to report that permission is denied. It is satisfied by [ErrPermission] as well as some syscall errors.

This function predates [errors.Is]. It only supports errors returned by the os package. New code should use errors.Is(err, fs.ErrPermission).

**Parameters**

- `err: Error`

**Returns boolean**
