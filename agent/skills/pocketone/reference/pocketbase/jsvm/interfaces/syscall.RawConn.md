Source: https://pocketbase.io/jsvm/interfaces/syscall.RawConn.html

# Interface RawConn

A RawConn is a raw network connection.

**Hierarchy**

- RawConn

**Indexable**

```ts
[key: string]: any
```

## Methods

### control

```ts
control(f): void
```

Control invokes f on the underlying connection's file descriptor or handle. The file descriptor fd is guaranteed to remain valid while f executes but not after f returns.

**Parameters**

- `f: ((fd) => void)`

**Returns void**

### read

```ts
read(f): void
```

Read invokes f on the underlying connection's file descriptor or handle; f is expected to try to read from the file descriptor. If f returns true, Read returns. Otherwise Read blocks waiting for the connection to be ready for reading and tries again repeatedly. The file descriptor is guaranteed to remain valid while f executes but not after f returns.

**Parameters**

- `f: ((fd) => boolean)`

**Returns void**

### write

```ts
write(f): void
```

Write is like Read but for writing.

**Parameters**

- `f: ((fd) => boolean)`

**Returns void**
