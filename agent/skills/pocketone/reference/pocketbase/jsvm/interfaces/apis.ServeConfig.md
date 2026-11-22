Source: https://pocketbase.io/jsvm/interfaces/apis.ServeConfig.html

# Interface ServeConfig

ServeConfig defines a configuration struct for apis.Serve().

**Hierarchy**

- ServeConfig

## Properties

### allowedOrigins

```ts
allowedOrigins: string[]
```

AllowedOrigins is an optional list of CORS origins (default to "*").

### certificateDomains

```ts
certificateDomains: string[]
```

Optional domains list to use when issuing the TLS certificate.

If not set, the host from the bound server address will be used.

For convenience, for each "non-www" domain a "www" entry and redirect will be automatically added.

### httpAddr

```ts
httpAddr: string
```

HttpAddr is the TCP address to listen for the HTTP server (eg. "127.0.0.1:80").

### httpsAddr

```ts
httpsAddr: string
```

HttpsAddr is the TCP address to listen for the HTTPS server (eg. "127.0.0.1:443").

### showStartBanner

```ts
showStartBanner: boolean
```

ShowStartBanner indicates whether to show or hide the server start console message.
