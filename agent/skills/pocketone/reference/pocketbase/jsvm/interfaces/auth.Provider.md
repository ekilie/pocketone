Source: https://pocketbase.io/jsvm/interfaces/auth.Provider.html

# Interface Provider

### Todo

refactor and consider replace with a plain struct

Provider defines a common interface for an OAuth2 client.

**Hierarchy**

- Provider

**Indexable**

```ts
[key: string]: any
```

## Methods

### authURL

```ts
authURL(): string
```

AuthURL returns the provider's authorization service url.

**Returns string**

### buildAuthURL

```ts
buildAuthURL(state, ...opts): string
```

BuildAuthURL returns a URL to the provider's consent page that asks for permissions for the required scopes explicitly.

**Parameters**

- `state: string`
- `Rest ...opts: AuthCodeOption[]`

**Returns string**

### client

```ts
client(token): any
```

Client returns an http client using the provided token.

**Parameters**

- `token: Token`

**Returns any**

### clientId

```ts
clientId(): string
```

ClientId returns the provider client's app ID.

**Returns string**

### clientSecret

```ts
clientSecret(): string
```

ClientSecret returns the provider client's app secret.

**Returns string**

### context

```ts
context(): context.Context
```

Context returns the context associated with the provider (if any).

**Returns context.Context**

### displayName

```ts
displayName(): string
```

DisplayName usually returns provider name as it is officially written and it could be used directly in the UI.

**Returns string**

### extra

```ts
extra(): _TygojaDict
```

Extra returns a shallow copy of any custom config data that the provider may need.

**Returns _TygojaDict**

### fetchAuthUser

```ts
fetchAuthUser(token): AuthUser
```

FetchAuthUser is similar to FetchRawUserInfo, but normalizes and marshalizes the user api response into a standardized AuthUser struct.

**Parameters**

- `token: Token`

**Returns AuthUser**

### fetchRawUserInfo

```ts
fetchRawUserInfo(token): string | number[]
```

FetchRawUserInfo requests and marshalizes into `result` the the OAuth user api response.

**Parameters**

- `token: Token`

**Returns string | number[]**

### fetchToken

```ts
fetchToken(code, ...opts): Token
```

FetchToken converts an authorization code to token.

**Parameters**

- `code: string`
- `Rest ...opts: AuthCodeOption[]`

**Returns Token**

### logo

```ts
logo(): string
```

**Returns string**

### Todo

temp backport

Order returns the sorting order of the provider usually used in the auth methods list response.

### order

```ts
order(): number
```

**Returns number**

### Todo

temp backport

Order returns the sorting order of the provider usually used in the auth methods list response.

### pkce

```ts
pkce(): boolean
```

PKCE indicates whether the provider can use the PKCE flow.

**Returns boolean**

### redirectURL

```ts
redirectURL(): string
```

RedirectURL returns the end address to redirect the user going through the OAuth flow.

**Returns string**

### scopes

```ts
scopes(): string[]
```

Scopes returns the provider access permissions that will be requested.

**Returns string[]**

### setAuthURL

```ts
setAuthURL(url): void
```

SetAuthURL sets the provider's AuthURL.

**Parameters**

- `url: string`

**Returns void**

### setClientId

```ts
setClientId(clientId): void
```

SetClientId sets the provider client's ID.

**Parameters**

- `clientId: string`

**Returns void**

### setClientSecret

```ts
setClientSecret(secret): void
```

SetClientSecret sets the provider client's app secret.

**Parameters**

- `secret: string`

**Returns void**

### setContext

```ts
setContext(ctx): void
```

SetContext assigns the specified context to the current provider.

**Parameters**

- `ctx: context.Context`

**Returns void**

### setDisplayName

```ts
setDisplayName(displayName): void
```

SetDisplayName sets the provider's display name.

**Parameters**

- `displayName: string`

**Returns void**

### setExtra

```ts
setExtra(data): void
```

SetExtra updates the provider's custom config data.

**Parameters**

- `data: _TygojaDict`

**Returns void**

### setPKCE

```ts
setPKCE(enable): void
```

SetPKCE toggles the state whether the provider can use the PKCE flow or not.

**Parameters**

- `enable: boolean`

**Returns void**

### setRedirectURL

```ts
setRedirectURL(url): void
```

SetRedirectURL sets the provider's RedirectURL.

**Parameters**

- `url: string`

**Returns void**

### setScopes

```ts
setScopes(scopes): void
```

SetScopes sets the provider access permissions that will be requested later.

**Parameters**

- `scopes: string[]`

**Returns void**

### setTokenURL

```ts
setTokenURL(url): void
```

SetTokenURL sets the provider's TokenURL.

**Parameters**

- `url: string`

**Returns void**

### setUserInfoURL

```ts
setUserInfoURL(url): void
```

SetUserInfoURL sets the provider's UserInfoURL.

**Parameters**

- `url: string`

**Returns void**

### tokenURL

```ts
tokenURL(): string
```

TokenURL returns the provider's token exchange service url.

**Returns string**

### userInfoURL

```ts
userInfoURL(): string
```

UserInfoURL returns the provider's user info api url.

**Returns string**
