Source: https://pocketbase.io/jsvm/interfaces/apis.gzipResponseWriter.html

# Interface gzipResponseWriter

**Hierarchy**

- [_sJnSXiw](https://pocketbase.io/jsvm/types/apis._sJnSXiw.html)

  - gzipResponseWriter

## Methods

### flush

```ts
flush(): void
```

**Returns void**

### header

```ts
header(): Header
```

Header returns the header map that will be sent by [ResponseWriter.WriteHeader]. The [Header] map also is the mechanism with which [Handler] implementations can set HTTP trailers.

Changing the header map after a call to [ResponseWriter.WriteHeader] (or [ResponseWriter.Write]) has no effect unless the HTTP status code was of the 1xx class or the modified headers are trailers.

There are two ways to set Trailers. The preferred way is to predeclare in the headers which trailers you will later send by setting the "Trailer" header to the names of the trailer keys which will come later. In this case, those keys of the Header map are treated as if they were trailers. See the example. The second way, for trailer keys not known to the [Handler] until after the first [ResponseWriter.Write], is to prefix the [Header] map keys with the [TrailerPrefix] constant value.

To suppress automatic response headers (such as "Date"), set their value to nil.

**Returns Header**

### hijack

```ts
hijack(): [net.Conn, ReadWriter]
```

**Returns [net.Conn, ReadWriter]**

### push

```ts
push(target, opts): void
```

**Parameters**

- `target: string`
- `opts: PushOptions`

**Returns void**

### unwrap

```ts
unwrap(): ResponseWriter
```

**Returns ResponseWriter**

### write

```ts
write(b): number
```

**Parameters**

- `b: string | number[]`

**Returns number**

### writeHeader

```ts
writeHeader(code): void
```

**Parameters**

- `code: number`

**Returns void**
