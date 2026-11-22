Source: https://pocketbase.io/jsvm/interfaces/os.openFile.html

# Interface openFile

**Hierarchy**

- openFile

```ts
openFile(name, flag, perm): os.File
```

OpenFile is the generalized open call; most users will use Open or Create instead. It opens the named file with specified flag ([O_RDONLY] etc.). If the file does not exist, and the [O_CREATE] flag is passed, it is created with mode perm (before umask); the containing directory must exist. If successful, methods on the returned File can be used for I/O. If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`
- `flag: number`
- `perm: os.FileMode`

**Returns os.File**
