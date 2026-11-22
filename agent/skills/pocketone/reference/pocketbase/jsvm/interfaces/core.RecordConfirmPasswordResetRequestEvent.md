Source: https://pocketbase.io/jsvm/interfaces/core.RecordConfirmPasswordResetRequestEvent.html

# Interface RecordConfirmPasswordResetRequestEvent

**Hierarchy**

- [_sSEurOr](https://pocketbase.io/jsvm/types/core._sSEurOr.html)

  - RecordConfirmPasswordResetRequestEvent

## Methods

### badRequestError

```ts
badRequestError(message, errData): router.ApiError
```

**Parameters**

- `message: string`
- `errData: any`

**Returns router.ApiError**

### bindBody

```ts
bindBody(dst): void
```

BindBody unmarshal the request body into the provided dst.

dst must be either a struct pointer or map[string]any.

The rules how the body will be scanned depends on the request Content-Type.

Currently the following Content-Types are supported:

```
  - application/json
  - text/xml, application/xml
  - multipart/form-data, application/x-www-form-urlencoded
```

Respectively the following struct tags are supported (again, which one will be used depends on the Content-Type):

```
  - "json" (json body)- uses the builtin Go json package for unmarshaling.
  - "xml" (xml body) - uses the builtin Go xml package for unmarshaling.
  - "form" (form data) - utilizes the custom [router.UnmarshalRequestData] method.
```

NB! When dst is a struct make sure that it doesn't have public fields that shouldn't be bindable and it is advisible such fields to be unexported or have a separate struct just for the binding. For example:

```
 data := struct{
    somethingPrivate string

    Title string `json:"title" form:"title"`
    Total int    `json:"total" form:"total"`
 }{}
 err := e.BindBody(&data)
```

**Parameters**

- `dst: any`

**Returns void**

### blob

```ts
blob(status, contentType, b): void
```

Blob writes a blob (bytes slice) response.

**Parameters**

- `status: number`
- `contentType: string`
- `b: string | number[]`

**Returns void**

### error

```ts
error(status, message, errData): router.ApiError
```

**Parameters**

- `status: number`
- `message: string`
- `errData: any`

**Returns router.ApiError**

### fileFS

```ts
fileFS(fsys, filename): void
```

FileFS serves the specified filename from fsys.

It is similar to [echo.FileFS] for consistency with earlier versions.

**Parameters**

- `fsys: FS`
- `filename: string`

**Returns void**

### findUploadedFiles

```ts
findUploadedFiles(key): filesystem.File[]
```

FindUploadedFiles extracts all form files of "key" from a http request and returns a slice with filesystem.File instances (if any).

**Parameters**

- `key: string`

**Returns filesystem.File[]**

### flush

```ts
flush(): void
```

Flush flushes buffered data to the current response.

Returns [http.ErrNotSupported] if e.Response doesn't implement the [http.Flusher] interface (all router package handlers receives a ResponseWritter that implements it unless explicitly replaced with a custom one).

**Returns void**

### forbiddenError

```ts
forbiddenError(message, errData): router.ApiError
```

**Parameters**

- `message: string`
- `errData: any`

**Returns router.ApiError**

### get

```ts
get(key): any
```

Get retrieves single value from the current event data store.

**Parameters**

- `key: string`

**Returns any**

### getAll

```ts
getAll(): _TygojaDict
```

GetAll returns a copy of the current event data store.

**Returns _TygojaDict**

### hasSuperuserAuth

```ts
hasSuperuserAuth(): boolean
```

HasSuperuserAuth checks whether the current RequestEvent has superuser authentication loaded.

**Returns boolean**

### html

```ts
html(status, data): void
```

HTML writes an HTML response.

**Parameters**

- `status: number`
- `data: string`

**Returns void**

### internalServerError

```ts
internalServerError(message, errData): router.ApiError
```

**Parameters**

- `message: string`
- `errData: any`

**Returns router.ApiError**

### isTLS

```ts
isTLS(): boolean
```

IsTLS reports whether the connection on which the request was received is TLS.

**Returns boolean**

### json

```ts
json(status, data): void
```

JSON writes a JSON response.

It also provides a generic response data fields picker if the "fields" query parameter is set. For example, if you are requesting `?fields=a,b` for `e.JSON(200, map[string]int{ "a":1, "b":2, "c":3 })`, it should result in a JSON response like: `{"a":1, "b": 2}`.

Note that invalid UTF8 characters are mangled for compatibility with earlier versions and to prevent unnecessary causing a response error.

**Parameters**

- `status: number`
- `data: any`

**Returns void**

### next

```ts
next(): void
```

Next calls the next hook handler.

**Returns void**

### noContent

```ts
noContent(status): void
```

NoContent writes a response with no body (ex. 204).

**Parameters**

- `status: number`

**Returns void**

### notFoundError

```ts
notFoundError(message, errData): router.ApiError
```

**Parameters**

- `message: string`
- `errData: any`

**Returns router.ApiError**

### realIP

```ts
realIP(): string
```

RealIP returns the "real" IP address from the configured trusted proxy headers.

If Settings.TrustedProxy is not configured or the found IP is empty, it fallbacks to e.RemoteIP().

NB! Be careful when used in a security critical context as it relies on the trusted proxy to be properly configured and your app to be accessible only through it. If you are not sure, use e.RemoteIP().

**Returns string**

### redirect

```ts
redirect(status, url): void
```

Redirect writes a redirect response to the specified url. The status code must be in between 300 – 399 range.

**Parameters**

- `status: number`
- `url: string`

**Returns void**

### remoteIP

```ts
remoteIP(): string
```

RemoteIP returns the IP address of the client that sent the request.

IPv6 addresses are returned expanded. For example, "2001:db8::1" becomes "2001:0db8:0000:0000:0000:0000:0000:0001".

Note that if you are behind reverse proxy(ies), this method returns the IP of the last connecting proxy.

**Returns string**

### requestInfo

```ts
requestInfo(): RequestInfo
```

RequestInfo parses the current request into RequestInfo instance.

Note that the returned result is cached to avoid copying the request data multiple times but the auth state and other common store items are always refreshed in case they were changed by another handler.

**Returns RequestInfo**

### set

```ts
set(key, value): void
```

Set saves single value into the current event data store.

**Parameters**

- `key: string`
- `value: any`

**Returns void**

### setAll

```ts
setAll(m): void
```

SetAll saves all items from m into the current event data store.

**Parameters**

- `m: _TygojaDict`

**Returns void**

### setCookie

```ts
setCookie(cookie): void
```

SetCookie is an alias for [http.SetCookie].

SetCookie adds a Set-Cookie header to the current response's headers. The provided cookie must have a valid Name. Invalid cookies may be silently dropped.

**Parameters**

- `cookie: http.Cookie`

**Returns void**

### status

```ts
status(): number
```

Status reports the status code of the current response.

This method always returns 0 if e.Response doesn't implement the StatusTracker interface (all router package handlers receives a ResponseWritter that implements it unless explicitly replaced with a custom one).

**Returns number**

### stream

```ts
stream(status, contentType, reader): void
```

Stream streams the specified reader into the response.

**Parameters**

- `status: number`
- `contentType: string`
- `reader: io.Reader`

**Returns void**

### string

```ts
string(status, data): void
```

String writes a plain string response.

**Parameters**

- `status: number`
- `data: string`

**Returns void**

### tags

```ts
tags(): string[]
```

**Returns string[]**

### Todo

consider storing the original collection name and use that as a tag to avoid the ambiguity when the collection is being modified (#7613); for new collection also maybe return empty tags?

### tooManyRequestsError

```ts
tooManyRequestsError(message, errData): router.ApiError
```

**Parameters**

- `message: string`
- `errData: any`

**Returns router.ApiError**

### unauthorizedError

```ts
unauthorizedError(message, errData): router.ApiError
```

**Parameters**

- `message: string`
- `errData: any`

**Returns router.ApiError**

### written

```ts
written(): boolean
```

Written reports whether the current response has already been written.

This method always returns false if e.ResponseWritter doesn't implement the WriteTracker interface (all router package handlers receives a ResponseWritter that implements it unless explicitly replaced with a custom one).

**Returns boolean**

### xml

```ts
xml(status, data): void
```

XML writes an XML response. It automatically prepends the generic [xml.Header] string to the response.

**Parameters**

- `status: number`
- `data: any`

**Returns void**

## Properties

### app

```ts
app: App
```

### Optional auth

```ts
auth?: core.Record
```

### Optional collection

```ts
collection?: core.Collection
```

### extra

```ts
extra: _TygojaDict
```

Extra holds any dynamic data that could be passed between event handlers.

### Optional record

```ts
record?: core.Record
```

### Optional request

```ts
request?: Request
```

### response

```ts
response: ResponseWriter
```
