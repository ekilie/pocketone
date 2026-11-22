Source: https://pocketbase.io/jsvm/interfaces/auth.AuthUser.html

# Interface AuthUser

AuthUser defines a standardized OAuth2 user data structure.

**Hierarchy**

- AuthUser

## Methods

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements the [json.Marshaler] interface.

**Returns string | number[]**

### Todo

remove after dropping v0.22 support

## Properties

### accessToken

```ts
accessToken: string
```

### avatarURL

```ts
avatarURL: string
```

### avatarUrl

```ts
avatarUrl: string
```

### Todo

deprecated: use AvatarURL instead AvatarUrl will be removed after dropping v0.22 support

### email

```ts
email: string
```

The VERIFIED OAuth2 account email.

It must be empty if the provider is not able to verify the email ownership.

### expiry

```ts
expiry: types.DateTime
```

### id

```ts
id: string
```

### name

```ts
name: string
```

### rawUser

```ts
rawUser: _TygojaDict
```

### refreshToken

```ts
refreshToken: string
```

### username

```ts
username: string
```
