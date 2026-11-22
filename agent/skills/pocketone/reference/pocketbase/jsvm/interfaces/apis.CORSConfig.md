Source: https://pocketbase.io/jsvm/interfaces/apis.CORSConfig.html

# Interface CORSConfig

CORSConfig defines the config for CORS middleware.

**Hierarchy**

- CORSConfig

## Properties

### allowCredentials

```ts
allowCredentials: boolean
```

AllowCredentials determines the value of the Access-Control-Allow-Credentials response header. This header indicates whether or not the response to the request can be exposed when the credentials mode (Request.credentials) is true. When used as part of a response to a preflight request, this indicates whether or not the actual request can be made using credentials. See also [MDN: Access-Control-Allow-Credentials].

Optional. Default value false, in which case the header is not set.

Security: avoid using `AllowCredentials = true` with `AllowOrigins = *`. See "Exploiting CORS misconfigurations for Bitcoins and bounties", [https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html](https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html)

See also: [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Credentials](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Credentials)

### allowHeaders

```ts
allowHeaders: string[]
```

AllowHeaders determines the value of the Access-Control-Allow-Headers response header. This header is used in response to a preflight request to indicate which HTTP headers can be used when making the actual request.

Optional. Default value []string{}.

See also: [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Headers](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Headers)

### allowMethods

```ts
allowMethods: string[]
```

AllowMethods determines the value of the Access-Control-Allow-Methods response header. This header specified the list of methods allowed when accessing the resource. This is used in response to a preflight request.

Optional. Default value DefaultCORSConfig.AllowMethods.

See also: [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Methods](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Methods)

### allowOriginFunc

```ts
allowOriginFunc: ((origin) => boolean)
```

**Type declaration**

```ts
(origin): boolean
```

AllowOriginFunc is a custom function to validate the origin. It takes the origin as an argument and returns true if allowed or false otherwise. If an error is returned, it is returned by the handler. If this option is set, AllowOrigins is ignored.

Security: use extreme caution when handling the origin, and carefully validate any logic. Remember that attackers may register hostile domain names. See [https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html](https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html)

Optional.

**Parameters**

- `origin: string`

**Returns boolean**

### allowOrigins

```ts
allowOrigins: string[]
```

AllowOrigins determines the value of the Access-Control-Allow-Origin response header. This header defines a list of origins that may access the resource.

The wildcard characters '*' and '?' are supported as subdomain segments and are converted to regex fragments '.*' and '.' accordingly.

Security: use extreme caution when handling the origin, and carefully validate any logic. Remember that attackers may register hostile domain names. See [https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html](https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html)

Optional. Default value []string{"*"}.

See also: [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Origin](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Origin)

### exposeHeaders

```ts
exposeHeaders: string[]
```

ExposeHeaders determines the value of Access-Control-Expose-Headers, which defines a list of headers that clients are allowed to access.

Optional. Default value []string{}, in which case the header is not set.

See also: [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Expose-Header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Expose-Header)

### maxAge

```ts
maxAge: number
```

MaxAge determines the value of the Access-Control-Max-Age response header. This header indicates how long (in seconds) the results of a preflight request can be cached. The header is set only if MaxAge != 0, negative value sends "0" which instructs browsers not to cache that response.

Optional. Default value 0 - meaning header is not sent.

See also: [https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Max-Age](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Max-Age)

### unsafeWildcardOriginWithAllowCredentials

```ts
unsafeWildcardOriginWithAllowCredentials: boolean
```

UnsafeWildcardOriginWithAllowCredentials UNSAFE/INSECURE: allows wildcard '*' origin to be used with AllowCredentials flag. In that case we consider any origin allowed and send it back to the client with `Access-Control-Allow-Origin` header.

This is INSECURE and potentially leads to [cross-origin](https://portswigger.net/research/exploiting-cors-misconfigurations-for-bitcoins-and-bounties) attacks. See: [https://github.com/labstack/echo/issues/2400](https://github.com/labstack/echo/issues/2400) for discussion on the subject.

Optional. Default value is false.
