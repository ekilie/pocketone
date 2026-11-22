Source: https://pocketbase.io/jsvm/interfaces/http.Response.html

# Interface Response

Response represents the response from an HTTP request.

The [Client] and [Transport] return Responses from servers once the response headers have been received. The response body is streamed on demand as the Body field is read.

**Hierarchy**

- Response

## Methods

### cookies

```ts
cookies(): http.Cookie[]
```

Cookies parses and returns the cookies set in the Set-Cookie headers.

**Returns http.Cookie[]**

### location

```ts
location(): URL
```

Location returns the URL of the response's "Location" header, if present. Relative redirects are resolved relative to [Response.Request]. [ErrNoLocation] is returned if no Location header is present.

**Returns URL**

### protoAtLeast

```ts
protoAtLeast(major, minor): boolean
```

ProtoAtLeast reports whether the HTTP protocol used in the response is at least major.minor.

**Parameters**

- `major: number`
- `minor: number`

**Returns boolean**

### write

```ts
write(w): void
```

Write writes r to w in the HTTP/1.x server response format, including the status line, headers, body, and optional trailer.

This method consults the following fields of the response r:

```
 StatusCode
 ProtoMajor
 ProtoMinor
 Request.Method
 TransferEncoding
 Trailer
 Body
 ContentLength
 Header, values for non-canonical keys will have unpredictable behavior
```

The Response Body is closed after it is sent.

**Parameters**

- `w: io.Writer`

**Returns void**

## Properties

### body

```ts
body: ReadCloser
```

Body represents the response body.

The response body is streamed on demand as the Body field is read. If the network connection fails or the server terminates the response, Body.Read calls return an error.

The http Client and Transport guarantee that Body is always non-nil, even on responses without a body or responses with a zero-length body. It is the caller's responsibility to close Body. The default HTTP client's Transport may not reuse HTTP/1.x "keep-alive" TCP connections if the Body is not read to completion and closed; however, manually reading the body to completion should not be needed in most cases, as closing the body will also cause the body to be read to completion asynchronously, up to a conservative limit.

The Body is automatically dechunked if the server replied with a "chunked" Transfer-Encoding.

As of Go 1.12, the Body will also implement io.Writer on a successful "101 Switching Protocols" response, as used by WebSockets and HTTP/2's "h2c" mode.

### close

```ts
close: boolean
```

Close records whether the header directed that the connection be closed after reading Body. The value is advice for clients: neither ReadResponse nor Response.Write ever closes a connection.

### contentLength

```ts
contentLength: number
```

ContentLength records the length of the associated content. The value -1 indicates that the length is unknown. Unless Request.Method is "HEAD", values >= 0 indicate that the given number of bytes may be read from Body.

### header

```ts
header: Header
```

Header maps header keys to values. If the response had multiple headers with the same key, they may be concatenated, with comma delimiters. (RFC 7230, section 3.2.2 requires that multiple headers be semantically equivalent to a comma-delimited sequence.) When Header values are duplicated by other fields in this struct (e.g., ContentLength, TransferEncoding, Trailer), the field values are authoritative.

Keys in the map are canonicalized (see CanonicalHeaderKey).

### proto

```ts
proto: string
```

### protoMajor

```ts
protoMajor: number
```

### protoMinor

```ts
protoMinor: number
```

### Optional request

```ts
request?: Request
```

Request is the request that was sent to obtain this Response. Request's Body is nil (having already been consumed). This is only populated for Client requests.

### status

```ts
status: string
```

### statusCode

```ts
statusCode: number
```

### Optional tls

```ts
tls?: any
```

TLS contains information about the TLS connection on which the response was received. It is nil for unencrypted responses. The pointer is shared between responses and should not be modified.

### trailer

```ts
trailer: Header
```

Trailer maps trailer keys to values in the same format as Header.

The Trailer initially contains only nil values, one for each key specified in the server's "Trailer" header value. Those values are not added to Header.

Trailer must not be accessed concurrently with Read calls on the Body.

After Body.Read has returned io.EOF, Trailer will contain any trailer values sent by the server.

### transferEncoding

```ts
transferEncoding: string[]
```

Contains transfer encodings from outer-most to inner-most. Value is nil, means that "identity" encoding is used.

### uncompressed

```ts
uncompressed: boolean
```

Uncompressed reports whether the response was sent compressed but was decompressed by the http package. When true, reading from Body yields the uncompressed content instead of the compressed content actually set from the server, ContentLength is set to -1, and the "Content-Length" and "Content-Encoding" fields are deleted from the responseHeader. To get the original response from the server, set Transport.DisableCompression to true.
