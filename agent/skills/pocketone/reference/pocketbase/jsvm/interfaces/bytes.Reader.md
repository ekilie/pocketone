Source: https://pocketbase.io/jsvm/interfaces/bytes.Reader.html

# Interface Reader

A Reader implements the [io.Reader], [io.ReaderAt], [io.WriterTo], [io.Seeker], [io.ByteScanner], and [io.RuneScanner] interfaces by reading from a byte slice. Unlike a [Buffer], a Reader is read-only and supports seeking. The zero value for Reader operates like a Reader of an empty slice.

**Hierarchy**

- Reader

## Methods

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
