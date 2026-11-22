Source: https://pocketbase.io/jsvm/interfaces/oauth2.Token.html

# Interface Token

Token represents the credentials used to authorize the requests to access protected resources on the OAuth 2.0 provider's backend.

Most users of this package should not access fields of Token directly. They're exported mostly for use by related packages implementing derivative OAuth2 flows.

**Hierarchy**

- Token

## Methods

### extra

```ts
extra(key): any
```

Extra returns an extra field. Extra fields are key-value pairs returned by the server as part of the token retrieval response.

**Parameters**

- `key: string`

**Returns any**

### setAuthHeader

```ts
setAuthHeader(r): void
```

SetAuthHeader sets the Authorization header to r using the access token in t.

This method is unnecessary when using [Transport] or an HTTP Client returned by this package.

**Parameters**

- `r: Request`

**Returns void**

### type

```ts
type(): string
```

Type returns t.TokenType if non-empty, else "Bearer".

**Returns string**

### valid

```ts
valid(): boolean
```

Valid reports whether t is non-nil, has an AccessToken, and is not expired.

**Returns boolean**

### withExtra

```ts
withExtra(extra): Token
```

WithExtra returns a new [Token] that's a clone of t, but using the provided raw extra map. This is only intended for use by packages implementing derivative OAuth2 flows.

**Parameters**

- `extra: any`

**Returns Token**

## Properties

### accessToken

```ts
accessToken: string
```

AccessToken is the token that authorizes and authenticates the requests.

### expiresIn

```ts
expiresIn: number
```

ExpiresIn is the OAuth2 wire format "expires_in" field, which specifies how many seconds later the token expires, relative to an unknown time base approximately around "now". It is the application's responsibility to populate `Expiry` from `ExpiresIn` when required.

### expiry

```ts
expiry: Time
```

Expiry is the optional expiration time of the access token.

If zero, [TokenSource] implementations will reuse the same token forever and RefreshToken or equivalent mechanisms for that TokenSource will not be used.

### refreshToken

```ts
refreshToken: string
```

RefreshToken is a token that's used by the application (as opposed to the user) to refresh the access token if it expires.

### tokenType

```ts
tokenType: string
```

TokenType is the type of token. The Type method returns either this or "Bearer", the default.
