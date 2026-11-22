Source: https://pocketbase.io/jsvm/interfaces/os.rawConn.html

# Interface rawConn

rawConn implements syscall.RawConn.

**Hierarchy**

- rawConn

## Methods

### control

```ts
control(f): void
```

**Parameters**

- `f: ((_arg0) => void)`

**Returns void**

### read

```ts
read(f): void
```

**Parameters**

- `f: ((_arg0) => boolean)`

**Returns void**

### write

```ts
write(f): void
```

**Parameters**

- `f: ((_arg0) => boolean)`

**Returns void**
