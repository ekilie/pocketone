Source: https://pocketbase.io/jsvm/interfaces/http.Protocols.html

# Interface Protocols

Protocols is a set of HTTP protocols. The zero value is an empty set of protocols.

The supported protocols are:

```
  - HTTP1 is the HTTP/1.0 and HTTP/1.1 protocols.
    HTTP1 is supported on both unsecured TCP and secured TLS connections.

  - HTTP2 is the HTTP/2 protocol over a TLS connection.

  - UnencryptedHTTP2 is the HTTP/2 protocol over an unsecured TCP connection.
```

**Hierarchy**

- Protocols

## Methods

### http1

```ts
http1(): boolean
```

HTTP1 reports whether p includes HTTP/1.

**Returns boolean**

### http2

```ts
http2(): boolean
```

HTTP2 reports whether p includes HTTP/2.

**Returns boolean**

### setHTTP1

```ts
setHTTP1(ok): void
```

SetHTTP1 adds or removes HTTP/1 from p.

**Parameters**

- `ok: boolean`

**Returns void**

### setHTTP2

```ts
setHTTP2(ok): void
```

SetHTTP2 adds or removes HTTP/2 from p.

**Parameters**

- `ok: boolean`

**Returns void**

### setUnencryptedHTTP2

```ts
setUnencryptedHTTP2(ok): void
```

SetUnencryptedHTTP2 adds or removes unencrypted HTTP/2 from p.

**Parameters**

- `ok: boolean`

**Returns void**

### string

```ts
string(): string
```

**Returns string**

### unencryptedHTTP2

```ts
unencryptedHTTP2(): boolean
```

UnencryptedHTTP2 reports whether p includes unencrypted HTTP/2.

**Returns boolean**
