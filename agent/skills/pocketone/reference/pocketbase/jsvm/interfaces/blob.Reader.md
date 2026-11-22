Source: https://pocketbase.io/jsvm/interfaces/blob.Reader.html

# Interface Reader

Reader reads bytes from a blob. It implements io.ReadSeekCloser, and must be closed after reads are finished.

**Hierarchy**

- Reader

## Methods

### close

```ts
close(): void
```

Close implements io.Closer ([https://golang.org/pkg/io/#Closer](https://golang.org/pkg/io/#Closer)).

**Returns void**

### contentType

```ts
contentType(): string
```

ContentType returns the MIME type of the blob.

**Returns string**

### modTime

```ts
modTime(): Time
```

ModTime returns the time the blob was last modified.

**Returns Time**

### read

```ts
read(p): number
```

Read implements io.Reader ([https://golang.org/pkg/io/#Reader](https://golang.org/pkg/io/#Reader)).

**Parameters**

- `p: string | number[]`

**Returns number**

### seek

```ts
seek(offset, whence): number
```

Seek implements io.Seeker ([https://golang.org/pkg/io/#Seeker](https://golang.org/pkg/io/#Seeker)).

**Parameters**

- `offset: number`
- `whence: number`

**Returns number**

### size

```ts
size(): number
```

Size returns the size of the blob content in bytes.

**Returns number**

### writeTo

```ts
writeTo(w): number
```

WriteTo reads from r and writes to w until there's no more data or an error occurs. The return value is the number of bytes written to w.

It implements the io.WriterTo interface.

**Parameters**

- `w: io.Writer`

**Returns number**
