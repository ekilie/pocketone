Source: https://pocketbase.io/jsvm/interfaces/os.readlink.html

# Interface readlink

**Hierarchy**

- readlink

```ts
readlink(name): string
```

Readlink returns the destination of the named symbolic link. If there is an error, it will be of type [*PathError].

If the link destination is relative, Readlink returns the relative path without resolving it to an absolute one.

**Parameters**

- `name: string`

**Returns string**
