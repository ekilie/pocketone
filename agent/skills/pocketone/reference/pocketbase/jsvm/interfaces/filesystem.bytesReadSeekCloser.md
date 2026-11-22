Source: https://pocketbase.io/jsvm/interfaces/filesystem.bytesReadSeekCloser.html

# Interface bytesReadSeekCloser

**Hierarchy**

- [_srkKyUT](https://pocketbase.io/jsvm/types/filesystem._srkKyUT.html)

  - bytesReadSeekCloser

## Methods

### close

```ts
close(): void
```

Close implements the [io.ReadSeekCloser] interface.

**Returns void**

### len

```ts
len(): number
```

Len returns the number of bytes of the unread portion of the slice.

**Returns number**

### read

```ts
read(b): number
```

Read implements the [io.Reader] interface.

**Parameters**

- `b: string | number[]`

**Returns number**

### readAt

```ts
readAt(b, off): number
```

ReadAt implements the [io.ReaderAt] interface.

**Parameters**

- `b: string | number[]`
- `off: number`

**Returns number**

### readByte

```ts
readByte(): number
```

ReadByte implements the [io.ByteReader] interface.

**Returns number**

### readRune

```ts
readRune(): [number, number]
```

ReadRune implements the [io.RuneReader] interface.

**Returns [number, number]**

### reset

```ts
reset(b): void
```

Reset resets the [Reader] to be reading from b.

**Parameters**

- `b: string | number[]`

**Returns void**

### seek

```ts
seek(offset, whence): number
```

Seek implements the [io.Seeker] interface.

**Parameters**

- `offset: number`
- `whence: number`

**Returns number**

### size

```ts
size(): number
```

Size returns the original length of the underlying byte slice. Size is the number of bytes available for reading via [Reader.ReadAt]. The result is unaffected by any method calls except [Reader.Reset].

**Returns number**

### unreadByte

```ts
unreadByte(): void
```

UnreadByte complements [Reader.ReadByte] in implementing the [io.ByteScanner] interface.

**Returns void**

### unreadRune

```ts
unreadRune(): void
```

UnreadRune complements [Reader.ReadRune] in implementing the [io.RuneScanner] interface.

**Returns void**

### writeTo

```ts
writeTo(w): number
```

WriteTo implements the [io.WriterTo] interface.

**Parameters**

- `w: io.Writer`

**Returns number**
