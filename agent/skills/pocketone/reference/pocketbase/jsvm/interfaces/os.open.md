Source: https://pocketbase.io/jsvm/interfaces/os.open.html

# Interface open

**Hierarchy**

- open

```ts
open(name): os.File
```

Open opens the named file for reading. If successful, methods on the returned file can be used for reading; the associated file descriptor has mode [O_RDONLY]. If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`

**Returns os.File**
