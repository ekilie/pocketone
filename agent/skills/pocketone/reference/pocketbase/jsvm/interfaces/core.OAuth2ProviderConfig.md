Source: https://pocketbase.io/jsvm/interfaces/core.OAuth2ProviderConfig.html

# Interface OAuth2ProviderConfig

**Hierarchy**

- OAuth2ProviderConfig

## Methods

### initProvider

```ts
initProvider(): Provider
```

InitProvider returns a new auth.Provider instance loaded with the current OAuth2ProviderConfig options.

**Returns Provider**

### validate

```ts
validate(): void
```

Validate makes OAuth2ProviderConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### authURL

```ts
authURL: string
```

### clientId

```ts
clientId: string
```

### clientSecret

```ts
clientSecret: string
```

### displayName

```ts
displayName: string
```

### extra

```ts
extra: _TygojaDict
```

### name

```ts
name: string
```

### Optional pkce

```ts
pkce?: boolean
```

PKCE overwrites the default provider PKCE config option.

This usually shouldn't be needed but some OAuth2 vendors, like the LinkedIn OIDC, may require manual adjustment due to returning error if extra parameters are added to the request ([https://ganigeorgiev.com/pocketbase/discussions/3799#discussioncomment-7640312](https://ganigeorgiev.com/pocketbase/discussions/3799#discussioncomment-7640312))

### tokenURL

```ts
tokenURL: string
```

### userInfoURL

```ts
userInfoURL: string
```
