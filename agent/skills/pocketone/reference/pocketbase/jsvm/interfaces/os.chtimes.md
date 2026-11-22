Source: https://pocketbase.io/jsvm/interfaces/os.chtimes.html

# Interface chtimes

**Hierarchy**

- chtimes

```ts
chtimes(name, atime, mtime): void
```

Chtimes changes the access and modification times of the named file, similar to the Unix utime() or utimes() functions. A zero [time.Time] value will leave the corresponding file time unchanged.

The underlying filesystem may truncate or round the values to a less precise time unit. If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`
- `atime: Time`
- `mtime: Time`

**Returns void**
