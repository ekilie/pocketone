Source: https://pocketbase.io/jsvm/interfaces/http.Server.html

# Interface Server

A Server defines parameters for running an HTTP server. The zero value for Server is a valid configuration.

**Hierarchy**

- Server

## Methods

### close

```ts
close(): void
```

Close immediately closes all active net.Listeners and any connections in state [StateNew], [StateActive], or [StateIdle]. For a graceful shutdown, use [Server.Shutdown].

Close does not attempt to close (and does not even know about) any hijacked connections, such as WebSockets.

Close returns any error returned from closing the [Server]'s underlying Listener(s).

**Returns void**

### listenAndServe

```ts
listenAndServe(): void
```

ListenAndServe listens on the TCP network address s.Addr and then calls [Serve] to handle requests on incoming connections. Accepted connections are configured to enable TCP keep-alives.

If s.Addr is blank, ":http" is used.

ListenAndServe always returns a non-nil error. After [Server.Shutdown] or [Server.Close], the returned error is [ErrServerClosed].

**Returns void**

### listenAndServeTLS

```ts
listenAndServeTLS(certFile, keyFile): void
```

ListenAndServeTLS listens on the TCP network address s.Addr and then calls [ServeTLS] to handle requests on incoming TLS connections. Accepted connections are configured to enable TCP keep-alives.

Filenames containing a certificate and matching private key for the server must be provided if neither the [Server]'s TLSConfig.Certificates nor TLSConfig.GetCertificate are populated. If the certificate is signed by a certificate authority, the certFile should be the concatenation of the server's certificate, any intermediates, and the CA's certificate.

If s.Addr is blank, ":https" is used.

ListenAndServeTLS always returns a non-nil error. After [Server.Shutdown] or [Server.Close], the returned error is [ErrServerClosed].

**Parameters**

- `certFile: string`
- `keyFile: string`

**Returns void**

### registerOnShutdown

```ts
registerOnShutdown(f): void
```

RegisterOnShutdown registers a function to call on [Server.Shutdown]. This can be used to gracefully shutdown connections that have undergone ALPN protocol upgrade or that have been hijacked. This function should start protocol-specific graceful shutdown, but should not wait for shutdown to complete.

**Parameters**

- `f: (() => void)`

**Returns void**

### serve

```ts
serve(l): void
```

Serve accepts incoming connections on the Listener l, creating a new service goroutine for each. The service goroutines read requests and then call s.Handler to reply to them.

HTTP/2 support is only enabled if the Listener returns [*tls.Conn] connections and they were configured with "h2" in the TLS Config.NextProtos.

Serve always returns a non-nil error and closes l. After [Server.Shutdown] or [Server.Close], the returned error is [ErrServerClosed].

**Parameters**

- `l: Listener`

**Returns void**

### serveTLS

```ts
serveTLS(l, certFile, keyFile): void
```

ServeTLS accepts incoming connections on the Listener l, creating a new service goroutine for each. The service goroutines perform TLS setup and then read requests, calling s.Handler to reply to them.

Files containing a certificate and matching private key for the server must be provided if neither the [Server]'s TLSConfig.Certificates, TLSConfig.GetCertificate nor config.GetConfigForClient are populated. If the certificate is signed by a certificate authority, the certFile should be the concatenation of the server's certificate, any intermediates, and the CA's certificate.

ServeTLS always returns a non-nil error. After [Server.Shutdown] or [Server.Close], the returned error is [ErrServerClosed].

**Parameters**

- `l: Listener`
- `certFile: string`
- `keyFile: string`

**Returns void**

### setKeepAlivesEnabled

```ts
setKeepAlivesEnabled(v): void
```

SetKeepAlivesEnabled controls whether HTTP keep-alives are enabled. By default, keep-alives are always enabled. Only very resource-constrained environments or servers in the process of shutting down should disable them.

**Parameters**

- `v: boolean`

**Returns void**

### shutdown

```ts
shutdown(ctx): void
```

Shutdown gracefully shuts down the server without interrupting any active connections. Shutdown works by first closing all open listeners, then closing all idle connections, and then waiting indefinitely for connections to return to idle and then shut down. If the provided context expires before the shutdown is complete, Shutdown returns the context's error, otherwise it returns any error returned from closing the [Server]'s underlying Listener(s).

When Shutdown is called, [Serve], [ServeTLS], [ListenAndServe], and [ListenAndServeTLS] immediately return [ErrServerClosed]. Make sure the program doesn't exit and waits instead for Shutdown to return.

Shutdown does not attempt to close nor wait for hijacked connections such as WebSockets. The caller of Shutdown should separately notify such long-lived connections of shutdown and wait for them to close, if desired. See [Server.RegisterOnShutdown] for a way to register shutdown notification functions.

Once Shutdown has been called on a server, it may not be reused; future calls to methods such as Serve will return ErrServerClosed.

**Parameters**

- `ctx: context.Context`

**Returns void**

## Properties

### addr

```ts
addr: string
```

Addr optionally specifies the TCP address for the server to listen on, in the form "host:port". If empty, ":http" (port 80) is used. The service names are defined in RFC 6335 and assigned by IANA. See net.Dial for details of the address format.

### baseContext

```ts
baseContext: ((_arg0) => context.Context)
```

**Type declaration**

```ts
(_arg0): context.Context
```

BaseContext optionally specifies a function that returns the base context for incoming requests on this server. The provided Listener is the specific Listener that's about to start accepting requests. If BaseContext is nil, the default is context.Background(). If non-nil, it must return a non-nil context.

**Parameters**

- `_arg0: Listener`

**Returns context.Context**

### connContext

```ts
connContext: ((ctx, c) => context.Context)
```

**Type declaration**

```ts
(ctx, c): context.Context
```

ConnContext optionally specifies a function that modifies the context used for a new connection c. The provided ctx is derived from the base context and has a ServerContextKey value.

**Parameters**

- `ctx: context.Context`
- `c: net.Conn`

**Returns context.Context**

### connState

```ts
connState: ((_arg0, _arg1) => void)
```

**Type declaration**

```ts
(_arg0, _arg1): void
```

ConnState specifies an optional callback function that is called when a client connection changes state. See the ConnState type and associated constants for details.

**Parameters**

- `_arg0: net.Conn`
- `_arg1: ConnState`

**Returns void**

### disableClientPriority

```ts
disableClientPriority: boolean
```

DisableClientPriority specifies whether client-specified priority, as specified in RFC 9218, should be respected or not.

This field only takes effect if using HTTP/2, and if no custom write scheduler is defined for the HTTP/2 server. Otherwise, this field is a no-op.

If set to true, requests will be served in a round-robin manner, without prioritization.

### disableGeneralOptionsHandler

```ts
disableGeneralOptionsHandler: boolean
```

DisableGeneralOptionsHandler, if true, passes "OPTIONS *" requests to the Handler, otherwise responds with 200 OK and Content-Length: 0.

### Optional errorLog

```ts
errorLog?: any
```

ErrorLog specifies an optional logger for errors accepting connections, unexpected behavior from handlers, and underlying FileSystem errors. If nil, logging is done via the log package's standard logger.

### handler

```ts
handler: http.Handler
```

### Optional http2

```ts
http2?: HTTP2Config
```

HTTP2 configures HTTP/2 connections.

### idleTimeout

```ts
idleTimeout: Duration
```

IdleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled. If zero, the value of ReadTimeout is used. If negative, or if zero and ReadTimeout is zero or negative, there is no timeout.

### maxHeaderBytes

```ts
maxHeaderBytes: number
```

MaxHeaderBytes controls the maximum number of bytes the server will read parsing the request header's keys and values, including the request line. It does not limit the size of the request body. If zero, DefaultMaxHeaderBytes is used.

### maxHeaderValueCount

```ts
maxHeaderValueCount: number
```

MaxHeaderValueCount controls the maximum number of header values that the server is willing to parse from a request. If zero, DefaultMaxHeaderValueCount is used. Comma-separated values in a single header line are counted once, while values sent as multiple header lines are counted multiple times. An exception to this is the Trailer header, whose comma-separated values are counted separately, as each of them is expected to be received later as an individual trailer header line.

### Optional protocols

```ts
protocols?: Protocols
```

Protocols is the set of protocols accepted by the server.

If Protocols includes UnencryptedHTTP2, the server will accept unencrypted HTTP/2 connections. The server can serve both HTTP/1 and unencrypted HTTP/2 on the same address and port.

If Protocols is nil, the default is usually HTTP/1 and HTTP/2. If TLSNextProto is non-nil and does not contain an "h2" entry, the default is HTTP/1 only.

### readHeaderTimeout

```ts
readHeaderTimeout: Duration
```

ReadHeaderTimeout is the amount of time allowed to read request headers. The connection's read deadline is reset after reading the headers and the Handler can decide what is considered too slow for the body. If zero, the value of ReadTimeout is used. If negative, or if zero and ReadTimeout is zero or negative, there is no timeout.

### readTimeout

```ts
readTimeout: Duration
```

ReadTimeout is the maximum duration for reading the entire request, including the body. A zero or negative value means there will be no timeout.

Because ReadTimeout does not let Handlers make per-request decisions on each request body's acceptable deadline or upload rate, most users will prefer to use ReadHeaderTimeout. It is valid to use them both.

### Optional tlsConfig

```ts
tlsConfig?: any
```

TLSConfig optionally provides a TLS configuration for use by ServeTLS and ListenAndServeTLS. Note that this value is cloned by ServeTLS and ListenAndServeTLS, so it's not possible to modify the configuration with methods like tls.Config.SetSessionTicketKeys. To use SetSessionTicketKeys, use Server.Serve with a TLS Listener instead.

### tlsNextProto

```ts
tlsNextProto: _TygojaDict
```

TLSNextProto optionally specifies a function to take over ownership of the provided TLS connection when an ALPN protocol upgrade has occurred. The map key is the protocol name negotiated. The Handler argument should be used to handle HTTP requests and will initialize the Request's TLS and RemoteAddr if not already set. The connection is automatically closed when the function returns. If TLSNextProto is not nil, HTTP/2 support is not enabled automatically.

Historically, TLSNextProto was used to disable HTTP/2 support. The Server.Protocols field now provides a simpler way to do this.

### writeTimeout

```ts
writeTimeout: Duration
```

WriteTimeout is the maximum duration before timing out writes of the response. It is reset whenever a new request's header is read. Like ReadTimeout, it does not let Handlers make decisions on a per-request basis. A zero or negative value means there will be no timeout.
