Source: https://pocketbase.io/jsvm/interfaces/bufio.ReadWriter.html

# Interface ReadWriter

ReadWriter stores pointers to a [Reader] and a [Writer]. It implements [io.ReadWriter].

**Hierarchy**

- [_shWfDbP](https://pocketbase.io/jsvm/types/bufio._shWfDbP.html)

  - ReadWriter

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

Buffered returns the number of bytes that can be read from the current buffer.

**Returns number**

### discard

```ts
discard(n): number
```

Discard skips the next n bytes, returning the number of bytes discarded.

If Discard skips fewer than n bytes, it also returns an error. If 0 <= n <= b.Buffered(), Discard is guaranteed to succeed without reading from the underlying io.Reader.

**Parameters**

- `n: number`

**Returns number**

### flush

```ts
flush(): void
```

Flush writes any buffered data to the underlying [io.Writer].

**Returns void**

### peek

```ts
peek(n): string | number[]
```

Peek returns the next n bytes without advancing the reader. The bytes stop being valid at the next read call. If necessary, Peek will read more bytes into the buffer in order to make n bytes available. If Peek returns fewer than n bytes, it also returns an error explaining why the read is short. The error is [ErrBufferFull] if n is larger than b's buffer size.

Calling Peek prevents a [Reader.UnreadByte] or [Reader.UnreadRune] call from succeeding until the next read operation.

**Parameters**

- `n: number`

**Returns string | number[]**

### read

```ts
read(p): number
```

Read reads data into p. It returns the number of bytes read into p. The bytes are taken from at most one Read on the underlying [Reader], hence n may be less than len(p). To read exactly len(p) bytes, use io.ReadFull(b, p). If the underlying [Reader] can return a non-zero count with io.EOF, then this Read method can do so as well; see the [io.Reader] docs.

**Parameters**

- `p: string | number[]`

**Returns number**

### readByte

```ts
readByte(): number
```

ReadByte reads and returns a single byte. If no byte is available, returns an error.

**Returns number**

### readBytes

```ts
readBytes(delim): string | number[]
```

ReadBytes reads until the first occurrence of delim in the input, returning a slice containing the data up to and including the delimiter. If ReadBytes encounters an error before finding a delimiter, it returns the data read before the error and the error itself (often io.EOF). ReadBytes returns err != nil if and only if the returned data does not end in delim. For simple uses, a Scanner may be more convenient.

**Parameters**

- `delim: number`

**Returns string | number[]**

### readFrom

```ts
readFrom(r): number
```

ReadFrom implements [io.ReaderFrom]. If the underlying writer supports the ReadFrom method, this calls the underlying ReadFrom. If there is buffered data and an underlying ReadFrom, this fills the buffer and writes it before calling ReadFrom.

**Parameters**

- `r: io.Reader`

**Returns number**

### readLine

```ts
readLine(): [string | number[], boolean]
```

ReadLine is a low-level line-reading primitive. Most callers should use [Reader.ReadBytes](https://pocketbase.io/jsvm/interfaces/'%5Cn') or [Reader.ReadString](https://pocketbase.io/jsvm/interfaces/'%5Cn') instead or use a [Scanner].

ReadLine tries to return a single line, not including the end-of-line bytes. If the line was too long for the buffer then isPrefix is set and the beginning of the line is returned. The rest of the line will be returned from future calls. isPrefix will be false when returning the last fragment of the line. The returned buffer is only valid until the next call to ReadLine. ReadLine either returns a non-nil line or it returns an error, never both.

The text returned from ReadLine does not include the line end ("\r\n" or "\n"). No indication or error is given if the input ends without a final line end. Calling [Reader.UnreadByte] after ReadLine will always unread the last byte read (possibly a character belonging to the line end) even if that byte is not part of the line returned by ReadLine.

**Returns [string | number[], boolean]**

### readRune

```ts
readRune(): [number, number]
```

ReadRune reads a single UTF-8 encoded Unicode character and returns the rune and its size in bytes. If the encoded rune is invalid, it consumes one byte and returns unicode.ReplacementChar (U+FFFD) with a size of 1.

**Returns [number, number]**

### readSlice

```ts
readSlice(delim): string | number[]
```

ReadSlice reads until the first occurrence of delim in the input, returning a slice pointing at the bytes in the buffer. The bytes stop being valid at the next read. If ReadSlice encounters an error before finding a delimiter, it returns all the data in the buffer and the error itself (often io.EOF). ReadSlice fails with error [ErrBufferFull] if the buffer fills without a delim. Because the data returned from ReadSlice will be overwritten by the next I/O operation, most clients should use [Reader.ReadBytes] or ReadString instead. ReadSlice returns err != nil if and only if line does not end in delim.

**Parameters**

- `delim: number`

**Returns string | number[]**

### readString

```ts
readString(delim): string
```

ReadString reads until the first occurrence of delim in the input, returning a string containing the data up to and including the delimiter. If ReadString encounters an error before finding a delimiter, it returns the data read before the error and the error itself (often io.EOF). ReadString returns err != nil if and only if the returned data does not end in delim. For simple uses, a Scanner may be more convenient.

**Parameters**

- `delim: number`

**Returns string**

### reset

```ts
reset(r): void
```

Reset discards any buffered data, resets all state, and switches the buffered reader to read from r. Calling Reset on the zero value of [Reader] initializes the internal buffer to the default size. Calling b.Reset(b) (that is, resetting a [Reader] to itself) does nothing.

**Parameters**

- `r: io.Reader`

**Returns void**

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

### unreadByte

```ts
unreadByte(): void
```

UnreadByte unreads the last byte. Only the most recently read byte can be unread.

UnreadByte returns an error if the most recent method called on the [Reader] was not a read operation. Notably, [Reader.Peek], [Reader.Discard], and [Reader.WriteTo] are not considered read operations.

**Returns void**

### unreadRune

```ts
unreadRune(): void
```

UnreadRune unreads the last rune. If the most recent method called on the [Reader] was not a [Reader.ReadRune], [Reader.UnreadRune] returns an error. (In this regard it is stricter than [Reader.UnreadByte], which will unread the last byte from any read operation.)

**Returns void**

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

### writeTo

```ts
writeTo(w): number
```

WriteTo implements io.WriterTo. This may make multiple calls to the [Reader.Read] method of the underlying [Reader]. If the underlying reader supports the [Reader.WriteTo] method, this calls the underlying [Reader.WriteTo] without buffering.

**Parameters**

- `w: io.Writer`

**Returns number**
