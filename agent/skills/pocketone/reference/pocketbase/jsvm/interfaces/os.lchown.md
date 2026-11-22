Source: https://pocketbase.io/jsvm/interfaces/os.lchown.html

# Interface lchown

**Hierarchy**

- lchown

```ts
lchown(name, uid, gid): void
```

Lchown changes the numeric uid and gid of the named file. If the file is a symbolic link, it changes the uid and gid of the link itself. If there is an error, it will be of type [*PathError].

On Windows, it always returns the [syscall.EWINDOWS] error, wrapped in [*PathError].

**Parameters**

- `name: string`
- `uid: number`
- `gid: number`

**Returns void**
