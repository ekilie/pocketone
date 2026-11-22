Source: https://pocketbase.io/jsvm/interfaces/http.HTTP2Config.html

# Interface HTTP2Config

HTTP2Config defines HTTP/2 configuration parameters common to both [Transport] and [Server].

**Hierarchy**

- HTTP2Config

## Properties

### countError

```ts
countError: ((errType) => void)
```

**Type declaration**

```ts
(errType): void
```

CountError, if non-nil, is called on HTTP/2 errors. It is intended to increment a metric for monitoring. The errType contains only lowercase letters, digits, and underscores (a-z, 0-9, _).

**Parameters**

- `errType: string`

**Returns void**

### maxConcurrentStreams

```ts
maxConcurrentStreams: number
```

MaxConcurrentStreams optionally specifies the number of concurrent streams that a client may have open at a time. If zero, MaxConcurrentStreams defaults to at least 100.

This parameter only applies to Servers.

### maxDecoderHeaderTableSize

```ts
maxDecoderHeaderTableSize: number
```

MaxDecoderHeaderTableSize optionally specifies an upper limit for the size of the header compression table used for decoding headers sent by the peer. A valid value is less than 4MiB. If zero or invalid, a default value is used.

### maxEncoderHeaderTableSize

```ts
maxEncoderHeaderTableSize: number
```

MaxEncoderHeaderTableSize optionally specifies an upper limit for the header compression table used for sending headers to the peer. A valid value is less than 4MiB. If zero or invalid, a default value is used.

### maxReadFrameSize

```ts
maxReadFrameSize: number
```

MaxReadFrameSize optionally specifies the largest frame this endpoint is willing to read. A valid value is between 16KiB and 16MiB, inclusive. If zero or invalid, a default value is used.

### maxReceiveBufferPerConnection

```ts
maxReceiveBufferPerConnection: number
```

MaxReceiveBufferPerConnection is the maximum size of the flow control window for data received on a connection. A valid value is at least 64KiB and less than 4MiB. If invalid, a default value is used.

### maxReceiveBufferPerStream

```ts
maxReceiveBufferPerStream: number
```

MaxReceiveBufferPerStream is the maximum size of the flow control window for data received on a stream (request). A valid value is less than 4MiB. If zero or invalid, a default value is used.

### permitProhibitedCipherSuites

```ts
permitProhibitedCipherSuites: boolean
```

PermitProhibitedCipherSuites, if true, permits the use of cipher suites prohibited by the HTTP/2 spec.

### pingTimeout

```ts
pingTimeout: Duration
```

PingTimeout is the timeout after which a connection will be closed if a response to a ping is not received. If zero, a default of 15 seconds is used.

### sendPingTimeout

```ts
sendPingTimeout: Duration
```

SendPingTimeout is the timeout after which a health check using a ping frame will be carried out if no frame is received on a connection. If zero, no health check is performed.

### strictMaxConcurrentRequests

```ts
strictMaxConcurrentRequests: boolean
```

StrictMaxConcurrentRequests controls whether an HTTP/2 server's concurrency limit should be respected across all connections to that server. If true, new requests sent when a connection's concurrency limit has been exceeded will block until an existing request completes. If false, an additional connection will be opened if all existing connections are at their limit.

This parameter only applies to Transports.

### writeByteTimeout

```ts
writeByteTimeout: Duration
```

WriteByteTimeout is the timeout after which a connection will be closed if no data can be written to it. The timeout begins when data is available to write, and is extended whenever any bytes are written.
