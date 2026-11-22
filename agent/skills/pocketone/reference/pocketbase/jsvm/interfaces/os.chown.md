Source: https://pocketbase.io/jsvm/interfaces/os.chown.html

# Interface chown

**Hierarchy**

- chown

```ts
chown(name, uid, gid): void
```

Chown changes the numeric uid and gid of the named file. If the file is a symbolic link, it changes the uid and gid of the link's target. A uid or gid of -1 means to not change that value. If there is an error, it will be of type [*PathError].

On Windows or Plan 9, Chown always returns the [syscall.EWINDOWS] or [syscall.EPLAN9] error, wrapped in [*PathError].

**Parameters**

- `name: string`
- `uid: number`
- `gid: number`

**Returns void**
