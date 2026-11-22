Source: https://pocketbase.io/jsvm/interfaces/http.Request.html

# Interface Request

A Request represents an HTTP request received by a server or to be sent by a client.

The field semantics differ slightly between client and server usage. In addition to the notes on the fields below, see the documentation for [Request.Write] and [RoundTripper].

**Hierarchy**

- Request

## Methods

### addCookie

```ts
addCookie(c): void
```

AddCookie adds a cookie to the request. Per RFC 6265 section 5.4, AddCookie does not attach more than one [Cookie] header field. That means all cookies, if any, are written into the same line, separated by semicolon. AddCookie only sanitizes c's name and value, and does not sanitize a Cookie header already present in the request.

**Parameters**

- `c: http.Cookie`

**Returns void**

### basicAuth

```ts
basicAuth(): [string, string, boolean]
```

BasicAuth returns the username and password provided in the request's Authorization header, if the request uses HTTP Basic Authentication. See RFC 2617, Section 2.

**Returns [string, string, boolean]**

### clone

```ts
clone(ctx): Request
```

Clone returns a deep copy of r with its context changed to ctx. The provided ctx must be non-nil.

Clone only makes a shallow copy of the Body field.

For an outgoing client request, the context controls the entire lifetime of a request and its response: obtaining a connection, sending the request, and reading the response headers and body.

**Parameters**

- `ctx: context.Context`

**Returns Request**

### context

```ts
context(): context.Context
```

Context returns the request's context. To change the context, use [Request.Clone] or [Request.WithContext].

The returned context is always non-nil; it defaults to the background context.

For outgoing client requests, the context controls cancellation.

For incoming server requests, the context is canceled when the client's connection closes, the request is canceled (with HTTP/2), or when the ServeHTTP method returns.

**Returns context.Context**

### cookie

```ts
cookie(name): http.Cookie
```

Cookie returns the named cookie provided in the request or [ErrNoCookie] if not found. If multiple cookies match the given name, only one cookie will be returned.

**Parameters**

- `name: string`

**Returns http.Cookie**

### cookies

```ts
cookies(): http.Cookie[]
```

Cookies parses and returns the HTTP cookies sent with the request.

**Returns http.Cookie[]**

### cookiesNamed

```ts
cookiesNamed(name): http.Cookie[]
```

CookiesNamed parses and returns the named HTTP cookies sent with the request or an empty slice if none matched.

**Parameters**

- `name: string`

**Returns http.Cookie[]**

### formFile

```ts
formFile(key): [multipart.File, FileHeader]
```

FormFile returns the first file for the provided form key. FormFile calls [Request.ParseMultipartForm] and [Request.ParseForm] if necessary.

**Parameters**

- `key: string`

**Returns [multipart.File, FileHeader]**

### formValue

```ts
formValue(key): string
```

FormValue returns the first value for the named component of the query. The precedence order:

1. application/x-www-form-urlencoded form body (POST, PUT, PATCH only)
2. query parameters (always)
3. multipart/form-data form body (always)

FormValue calls [Request.ParseMultipartForm] and [Request.ParseForm] if necessary and ignores any errors returned by these functions. If key is not present, FormValue returns the empty string. To access multiple values of the same key, call ParseForm and then inspect [Request.Form] directly.

**Parameters**

- `key: string`

**Returns string**

### multipartReader

```ts
multipartReader(): multipart.Reader
```

MultipartReader returns a MIME multipart reader if this is a multipart/form-data or a multipart/mixed POST request, else returns nil and an error. Use this function instead of [Request.ParseMultipartForm] to process the request body as a stream.

**Returns multipart.Reader**

### parseForm

```ts
parseForm(): void
```

ParseForm populates r.Form and r.PostForm.

For all requests, ParseForm parses the raw query from the URL and updates r.Form.

For POST, PUT, and PATCH requests, it also reads the request body, parses it as a form and puts the results into both r.PostForm and r.Form. Request body parameters take precedence over URL query string values in r.Form.

If the request Body's size has not already been limited by [MaxBytesReader], the size is capped at 10MB.

For other HTTP methods, or when the Content-Type is not application/x-www-form-urlencoded, the request Body is not read, and r.PostForm is initialized to a non-nil, empty value.

[Request.ParseMultipartForm] calls ParseForm automatically. ParseForm is idempotent.

**Returns void**

### parseMultipartForm

```ts
parseMultipartForm(maxMemory): void
```

ParseMultipartForm parses a request body as multipart/form-data. The whole request body is parsed and up to a total of maxMemory bytes of its file parts are stored in memory, with the remainder stored on disk in temporary files. ParseMultipartForm calls [Request.ParseForm] if necessary. If ParseForm returns an error, ParseMultipartForm returns it but also continues parsing the request body. After one call to ParseMultipartForm, subsequent calls have no effect.

**Parameters**

- `maxMemory: number`

**Returns void**

### pathValue

```ts
pathValue(name): string
```

PathValue returns the value for the named path wildcard in the [ServeMux] pattern that matched the request. It returns the empty string if the request was not matched against a pattern or there is no such wildcard in the pattern.

The value is unescaped. For example, if the pattern "/b/{bucket}" matches the path "/b/a%2fb", PathValue("bucket") returns "a/b".

**Parameters**

- `name: string`

**Returns string**

### postFormValue

```ts
postFormValue(key): string
```

PostFormValue returns the first value for the named component of the POST, PUT, or PATCH request body. URL query parameters are ignored. PostFormValue calls [Request.ParseMultipartForm] and [Request.ParseForm] if necessary and ignores any errors returned by these functions. If key is not present, PostFormValue returns the empty string.

**Parameters**

- `key: string`

**Returns string**

### protoAtLeast

```ts
protoAtLeast(major, minor): boolean
```

ProtoAtLeast reports whether the HTTP protocol used in the request is at least major.minor.

**Parameters**

- `major: number`
- `minor: number`

**Returns boolean**

### referer

```ts
referer(): string
```

Referer returns the referring URL, if sent in the request.

Referer is misspelled as in the request itself, a mistake from the earliest days of HTTP. This value can also be fetched from the [Header] map as Header["Referer"]; the benefit of making it available as a method is that the compiler can diagnose programs that use the alternate (correct English) spelling req.Referrer() but cannot diagnose programs that use Header["Referrer"].

**Returns string**

### setBasicAuth

```ts
setBasicAuth(username, password): void
```

SetBasicAuth sets the request's Authorization header to use HTTP Basic Authentication with the provided username and password.

With HTTP Basic Authentication the provided username and password are not encrypted. It should generally only be used in an HTTPS request.

The username may not contain a colon. Some protocols may impose additional requirements on pre-escaping the username and password. For instance, when used with OAuth2, both arguments must be URL encoded first with [url.QueryEscape].

**Parameters**

- `username: string`
- `password: string`

**Returns void**

### setPathValue

```ts
setPathValue(name, value): void
```

SetPathValue sets name to value, so that subsequent calls to r.PathValue(name) return value. It does not unescape value.

**Parameters**

- `name: string`
- `value: string`

**Returns void**

### userAgent

```ts
userAgent(): string
```

UserAgent returns the client's User-Agent, if sent in the request.

**Returns string**

### withContext

```ts
withContext(ctx): Request
```

WithContext returns a shallow copy of r with its context changed to ctx. The provided ctx must be non-nil.

For outgoing client request, the context controls the entire lifetime of a request and its response: obtaining a connection, sending the request, and reading the response headers and body.

To create a new request with a context, use [NewRequestWithContext]. To make a deep copy of a request with a new context, use [Request.Clone].

**Parameters**

- `ctx: context.Context`

**Returns Request**

### write

```ts
write(w): void
```

Write writes an HTTP/1.1 request, which is the header and body, in wire format. This method consults the following fields of the request:

```
 Host
 URL
 Method (defaults to "GET")
 Header
 ContentLength
 TransferEncoding
 Body
```

If Body is present, Content-Length is <= 0 and [Request.TransferEncoding] hasn't been set to "identity", Write adds "Transfer-Encoding: chunked" to the header. Body is closed after it is sent.

Header values for Host, Content-Length, Transfer-Encoding, and Trailer are not used; these are derived from other Request fields. If the Header does not contain a User-Agent value, Write uses "Go-http-client/1.1".

**Parameters**

- `w: io.Writer`

**Returns void**

### writeProxy

```ts
writeProxy(w): void
```

WriteProxy is like [Request.Write] but writes the request in the form expected by an HTTP proxy. In particular, [Request.WriteProxy] writes the initial Request-URI line of the request with an absolute URI, per section 5.3 of RFC 7230, including the scheme and host. In either case, WriteProxy also writes a Host header, using either r.Host or r.URL.Host.

**Parameters**

- `w: io.Writer`

**Returns void**

## Properties

### body

```ts
body: ReadCloser
```

Body is the request's body.

For client requests, a nil body means the request has no body, such as a GET request. The HTTP Client's Transport is responsible for calling the Close method.

For server requests, the Request Body is always non-nil but will return EOF immediately when no body is present. The Server will close the request body. The ServeHTTP Handler does not need to.

Body must allow Read to be called concurrently with Close. In particular, calling Close should unblock a Read waiting for input.

### cancel

```ts
cancel: undefined
```

Cancel is an optional channel whose closure indicates that the client request should be regarded as canceled. Not all implementations of RoundTripper may support Cancel.

For server requests, this field is not applicable.

Deprecated: Set the Request's context with NewRequestWithContext instead. If a Request's Cancel field and context are both set, it is undefined whether Cancel is respected.

### close

```ts
close: boolean
```

Close indicates whether to close the connection after replying to this request (for servers) or after sending this request and reading its response (for clients).

For server requests, the HTTP server handles this automatically and this field is not needed by Handlers.

For client requests, setting this field prevents re-use of TCP connections between requests to the same hosts, as if Transport.DisableKeepAlives were set.

### contentLength

```ts
contentLength: number
```

ContentLength records the length of the associated content. The value -1 indicates that the length is unknown. Values >= 0 indicate that the given number of bytes may be read from Body.

For client requests, a value of 0 with a non-nil Body is also treated as unknown.

### form

```ts
form: Values
```

Form contains the parsed form data, including both the URL field's query parameters and the PATCH, POST, or PUT form data. This field is only available after ParseForm is called. The HTTP client ignores Form and uses Body instead.

### getBody

```ts
getBody: (() => ReadCloser)
```

**Type declaration**

```ts
(): ReadCloser
```

GetBody defines an optional func to return a new copy of Body. It is used for client requests when a redirect requires reading the body more than once. Use of GetBody still requires setting Body.

For server requests, it is unused.

**Returns ReadCloser**

### header

```ts
header: Header
```

Header contains the request header fields either received by the server or to be sent by the client.

If a server received a request with header lines,

```
 Host: example.com
 accept-encoding: gzip, deflate
 Accept-Language: en-us
 fOO: Bar
 foo: two
```

then

```
 Header = map[string][]string{
   "Accept-Encoding": {"gzip, deflate"},
   "Accept-Language": {"en-us"},
   "Foo": {"Bar", "two"},
 }
```

For incoming requests, the Host header is promoted to the Request.Host field and removed from the Header map.

HTTP defines that header names are case-insensitive. The request parser implements this by using CanonicalHeaderKey, making the first character and any characters following a hyphen uppercase and the rest lowercase.

For client requests, certain headers such as Content-Length and Connection are automatically written when needed and values in Header may be ignored. See the documentation for the Request.Write method.

### host

```ts
host: string
```

For server requests, Host specifies the host on which the URL is sought. For HTTP/1 (per RFC 7230, section 5.4), this is either the value of the "Host" header or the host name given in the URL itself. For HTTP/2, it is the value of the ":authority" pseudo-header field. It may be of the form "host:port". For international domain names, Host may be in Punycode or Unicode form. Use golang.org/x/net/idna to convert it to either format if needed. To prevent DNS rebinding attacks, server Handlers should validate that the Host header has a value for which the Handler considers itself authoritative. The included ServeMux supports patterns registered to particular host names and thus protects its registered Handlers.

For client requests, Host optionally overrides the Host header to send. If empty, the Request.Write method uses the value of URL.Host. Host may contain an international domain name.

### method

```ts
method: string
```

Method specifies the HTTP method (GET, POST, PUT, etc.). For client requests, an empty string means GET.

### Optional multipartForm

```ts
multipartForm?: Form
```

MultipartForm is the parsed multipart form, including file uploads. This field is only available after ParseMultipartForm is called. The HTTP client ignores MultipartForm and uses Body instead.

### pattern

```ts
pattern: string
```

Pattern is the [ServeMux] pattern that matched the request. It is empty if the request was not matched against a pattern.

### postForm

```ts
postForm: Values
```

PostForm contains the parsed form data from PATCH, POST or PUT body parameters.

This field is only available after ParseForm is called. The HTTP client ignores PostForm and uses Body instead.

### proto

```ts
proto: string
```

The protocol version for incoming server requests.

For client requests, these fields are ignored. The HTTP client code always uses either HTTP/1.1 or HTTP/2. See the docs on Transport for details.

### protoMajor

```ts
protoMajor: number
```

### protoMinor

```ts
protoMinor: number
```

### remoteAddr

```ts
remoteAddr: string
```

RemoteAddr allows HTTP servers and other software to record the network address that sent the request, usually for logging. This field is not filled in by ReadRequest and has no defined format. The HTTP server in this package sets RemoteAddr to an "IP:port" address before invoking a handler. This field is ignored by the HTTP client.

### requestURI

```ts
requestURI: string
```

RequestURI is the unmodified request-target of the Request-Line (RFC 7230, Section 3.1.1) as sent by the client to a server. Usually the URL field should be used instead. It is an error to set this field in an HTTP client request.

### Optional response

```ts
response?: Response
```

Response is the redirect response which caused this request to be created. This field is only populated during client redirects.

### Optional tls

```ts
tls?: any
```

TLS allows HTTP servers and other software to record information about the TLS connection on which the request was received. This field is not filled in by ReadRequest. The HTTP server in this package sets the field for TLS-enabled connections before invoking a handler; otherwise it leaves the field nil. This field is ignored by the HTTP client.

### trailer

```ts
trailer: Header
```

Trailer specifies additional headers that are sent after the request body.

For server requests, the Trailer map initially contains only the trailer keys, with nil values. (The client declares which trailers it will later send.) While the handler is reading from Body, it must not reference Trailer. After reading from Body returns EOF, Trailer can be read again and will contain non-nil values, if they were sent by the client.

For client requests, Trailer must be initialized to a map containing the trailer keys to later send. The values may be nil or their final values. The ContentLength must be 0 or -1, to send a chunked request. After the HTTP request is sent the map values can be updated while the request body is read. Once the body returns EOF, the caller must not mutate Trailer.

Writing a request whose Trailer contains a key with invalid bytes (such as CR or LF), or such a value present when Write begins, returns an error.

Few HTTP clients, servers, or proxies support HTTP trailers.

### transferEncoding

```ts
transferEncoding: string[]
```

TransferEncoding lists the transfer encodings from outermost to innermost. An empty list denotes the "identity" encoding. TransferEncoding can usually be ignored; chunked encoding is automatically added and removed as necessary when sending and receiving requests.

### Optional url

```ts
url?: URL
```

URL specifies either the URI being requested (for server requests) or the URL to access (for client requests).

For server requests, the URL is parsed from the URI supplied on the Request-Line as stored in RequestURI. For most requests, fields other than Path and RawQuery will be empty. (See RFC 7230, Section 5.3)

For client requests, the URL's Host specifies the server to connect to, while the Request's Host field optionally specifies the Host header value to send in the HTTP request.
