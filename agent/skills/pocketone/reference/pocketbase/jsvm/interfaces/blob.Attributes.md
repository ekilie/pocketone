Source: https://pocketbase.io/jsvm/interfaces/blob.Attributes.html

# Interface Attributes

Attributes contains attributes about a blob.

**Hierarchy**

- Attributes

## Properties

### cacheControl

```ts
cacheControl: string
```

CacheControl specifies caching attributes that services may use when serving the blob. [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control)

### contentDisposition

```ts
contentDisposition: string
```

ContentDisposition specifies whether the blob content is expected to be displayed inline or as an attachment. [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition)

### contentEncoding

```ts
contentEncoding: string
```

ContentEncoding specifies the encoding used for the blob's content, if any. [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Encoding](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Encoding)

### contentLanguage

```ts
contentLanguage: string
```

ContentLanguage specifies the language used in the blob's content, if any. [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Language](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Language)

### contentType

```ts
contentType: string
```

ContentType is the MIME type of the blob. It will not be empty. [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type)

### createTime

```ts
createTime: Time
```

CreateTime is the time the blob was created, if available. If not available, CreateTime will be the zero time.

### eTag

```ts
eTag: string
```

ETag for the blob; see [https://en.wikipedia.org/wiki/HTTP_ETag](https://en.wikipedia.org/wiki/HTTP_ETag).

### md5

```ts
md5: string | number[]
```

MD5 is an MD5 hash of the blob contents or nil if not available.

### metadata

```ts
metadata: _TygojaDict
```

Metadata holds key/value pairs associated with the blob. Keys are guaranteed to be in lowercase, even if the backend service has case-sensitive keys (although note that Metadata written via this package will always be lowercased). If there are duplicate case-insensitive keys (e.g., "foo" and "FOO"), only one value will be kept, and it is undefined which one.

### modTime

```ts
modTime: Time
```

ModTime is the time the blob was last modified.

### size

```ts
size: number
```

Size is the size of the blob's content in bytes.
