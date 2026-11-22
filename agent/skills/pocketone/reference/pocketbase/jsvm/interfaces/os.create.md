Source: https://pocketbase.io/jsvm/interfaces/os.create.html

# Interface create

**Hierarchy**

- create

```ts
create(name): os.File
```

Create creates or truncates the named file. If the file already exists, it is truncated. If the file does not exist, it is created with mode 0o666 (before umask). If successful, methods on the returned File can be used for I/O; the associated file descriptor has mode [O_RDWR]. The directory containing the file must already exist. If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`

**Returns os.File**
