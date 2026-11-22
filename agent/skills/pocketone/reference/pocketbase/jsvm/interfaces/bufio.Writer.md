Source: https://pocketbase.io/jsvm/interfaces/bufio.Writer.html

# Interface Writer

Writer implements buffering for an [io.Writer] object. If an error occurs writing to a [Writer], no more data will be accepted and all subsequent writes, and [Writer.Flush], will return the error. After all data has been written, the client should call the [Writer.Flush] method to guarantee all data has been forwarded to the underlying [io.Writer].

**Hierarchy**

- Writer

## Methods

### available

```ts
available(): number
```

Available returns how many bytes are unused in the buffer.

**Returns number**

### availableBuffer

```ts
availableBuffer(): string | number[]
```

AvailableBuffer returns an empty buffer with b.Available() capacity. This buffer is intended to be appended to and passed to an immediately succeeding [Writer.Write] call. The buffer is only valid until the next write operation on b.

**Returns string | number[]**

### buffered

```ts
buffered(): number
```

Buffered returns the number of bytes that have been written into the current buffer.

**Returns number**

### flush

```ts
flush(): void
```

Flush writes any buffered data to the underlying [io.Writer].

**Returns void**

### readFrom

```ts
readFrom(r): number
```

ReadFrom implements [io.ReaderFrom]. If the underlying writer supports the ReadFrom method, this calls the underlying ReadFrom. If there is buffered data and an underlying ReadFrom, this fills the buffer and writes it before calling ReadFrom.

**Parameters**

- `r: io.Reader`

**Returns number**

### reset

```ts
reset(w): void
```

Reset discards any unflushed buffered data, clears any error, and resets b to write its output to w. Calling Reset on the zero value of [Writer] initializes the internal buffer to the default size. Calling w.Reset(w) (that is, resetting a [Writer] to itself) does nothing.

**Parameters**

- `w: io.Writer`

**Returns void**

### size

```ts
size(): number
```

Size returns the size of the underlying buffer in bytes.

**Returns number**

### write

```ts
write(p): number
```

Write writes the contents of p into the buffer. It returns the number of bytes written. If nn < len(p), it also returns an error explaining why the write is short.

**Parameters**

- `p: string | number[]`

**Returns number**

### writeByte

```ts
writeByte(c): void
```

WriteByte writes a single byte.

**Parameters**

- `c: number`

**Returns void**

### writeRune

```ts
writeRune(r): number
```

WriteRune writes a single Unicode code point, returning the number of bytes written and any error.

**Parameters**

- `r: number`

**Returns number**

### writeString

```ts
writeString(s): number
```

WriteString writes a string. It returns the number of bytes written. If the count is less than len(s), it also returns an error explaining why the write is short.

**Parameters**

- `s: string`

**Returns number**
