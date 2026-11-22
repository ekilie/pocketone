Source: https://pocketbase.io/jsvm/interfaces/apis.recordOAuth2LoginForm.html

# Interface recordOAuth2LoginForm

**Hierarchy**

- recordOAuth2LoginForm

## Properties

### code

```ts
code: string
```

The authorization code returned from the initial request.

### codeVerifier

```ts
codeVerifier: string
```

The optional PKCE code verifier as part of the code_challenge sent with the initial request.

### createData

```ts
createData: _TygojaDict
```

Additional data that will be used for creating a new auth record if an existing OAuth2 account doesn't exist.

### provider

```ts
provider: string
```

The name of the OAuth2 client provider (eg. "google")

### redirectURL

```ts
redirectURL: string
```

The redirect url sent with the initial request.

### redirectUrl

```ts
redirectUrl: string
```

### Todo

deprecated: use RedirectURL instead RedirectUrl will be removed after dropping v0.22 support
