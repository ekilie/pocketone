Source: https://pocketbase.io/jsvm/interfaces/apis.providerInfo.html

# Interface providerInfo

**Hierarchy**

- providerInfo

## Properties

### authURL

```ts
authURL: string
```

### authUrl

```ts
authUrl: string
```

### Todo

deprecated: use AuthURL instead AuthUrl will be removed after dropping v0.22 support

### codeChallenge

```ts
codeChallenge: string
```

### codeChallengeMethod

```ts
codeChallengeMethod: string
```

### codeVerifier

```ts
codeVerifier: string
```

technically could be omitted if the provider doesn't support PKCE, but to avoid breaking existing typed clients we'll return them as empty string

### displayName

```ts
displayName: string
```

### logo

```ts
logo: string
```

### name

```ts
name: string
```

### state

```ts
state: string
```
