Source: https://pocketbase.io/jsvm/interfaces/blob.WriterOptions.html

# Interface WriterOptions

WriterOptions sets options for NewWriter.

**Hierarchy**

- WriterOptions

## Properties

### bufferSize

```ts
bufferSize: number
```

BufferSize changes the default size in bytes of the chunks that Writer will upload in a single request; larger blobs will be split into multiple requests.

This option may be ignored by some drivers.

If 0, the driver will choose a reasonable default.

If the Writer is used to do many small writes concurrently, using a smaller BufferSize may reduce memory usage.

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

### contentMD5

```ts
contentMD5: string | number[]
```

ContentMD5 is used as a message integrity check. If len(ContentMD5) > 0, the MD5 hash of the bytes written must match ContentMD5, or Close will return an error without completing the write. [https://tools.ietf.org/html/rfc1864](https://tools.ietf.org/html/rfc1864)

### contentType

```ts
contentType: string
```

ContentType specifies the MIME type of the blob being written. If not set, it will be inferred from the content using the algorithm described at [http://mimesniff.spec.whatwg.org/](http://mimesniff.spec.whatwg.org/). Set DisableContentTypeDetection to true to disable the above and force the ContentType to stay empty. [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type)

### disableContentTypeDetection

```ts
disableContentTypeDetection: boolean
```

When true, if ContentType is the empty string, it will stay the empty string rather than being inferred from the content. Note that while the blob will be written with an empty string ContentType, most providers will fill one in during reads, so don't expect an empty ContentType if you read the blob back.

### maxConcurrency

```ts
maxConcurrency: number
```

MaxConcurrency changes the default concurrency for parts of an upload.

This option may be ignored by some drivers.

If 0, the driver will choose a reasonable default.

### metadata

```ts
metadata: _TygojaDict
```

Metadata holds key/value strings to be associated with the blob, or nil. Keys may not be empty, and are lowercased before being written. Duplicate case-insensitive keys (e.g., "foo" and "FOO") will result in an error.
