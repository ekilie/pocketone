Source: https://pocketbase.io/jsvm/interfaces/blob.Writer.html

# Interface Writer

Writer writes bytes to a blob.

It implements io.WriteCloser ([https://golang.org/pkg/io/#Closer](https://golang.org/pkg/io/#Closer)), and must be closed after all writes are done.

**Hierarchy**

- Writer

## Methods

### close

```ts
close(): void
```

Close closes the blob writer. The write operation is not guaranteed to have succeeded until Close returns with no error.

Close may return an error if the context provided to create the Writer is canceled or reaches its deadline.

**Returns void**

### readFrom

```ts
readFrom(r): number
```

ReadFrom reads from r and writes to w until EOF or error. The return value is the number of bytes read from r.

It implements the io.ReaderFrom interface.

**Parameters**

- `r: io.Reader`

**Returns number**

### write

```ts
write(p): number
```

Write implements the io.Writer interface ([https://golang.org/pkg/io/#Writer](https://golang.org/pkg/io/#Writer)).

Writes may happen asynchronously, so the returned error can be nil even if the actual write eventually fails. The write is only guaranteed to have succeeded if Close returns no error.

**Parameters**

- `p: string | number[]`

**Returns number**
