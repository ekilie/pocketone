Source: https://pocketbase.io/jsvm/interfaces/forms.AppleClientSecretCreate.html

# Interface AppleClientSecretCreate

AppleClientSecretCreate is a form struct to generate a new Apple Client Secret.

Reference: [https://developer.apple.com/documentation/sign_in_with_apple/generate_and_validate_tokens](https://developer.apple.com/documentation/sign_in_with_apple/generate_and_validate_tokens)

**Hierarchy**

- AppleClientSecretCreate

  - [AppleClientSecretCreateForm](https://pocketbase.io/jsvm/classes/AppleClientSecretCreateForm.html)

**Implemented by**

- [AppleClientSecretCreateForm](https://pocketbase.io/jsvm/classes/AppleClientSecretCreateForm.html)

## Methods

### submit

```ts
submit(): string
```

Submit validates the form and returns a new Apple Client Secret JWT.

**Returns string**

### validate

```ts
validate(): void
```

Validate makes the form validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### clientId

```ts
clientId: string
```

ClientId is the identifier of your app (aka. Service ID).

### duration

```ts
duration: number
```

Duration specifies how long the generated JWT should be considered valid. The specified value must be in seconds and max 15777000 (~6months).

### keyId

```ts
keyId: string
```

KeyId is a 10-character key identifier generated for the "Sign in with Apple" private key associated with your developer account.

### privateKey

```ts
privateKey: string
```

PrivateKey is the private key associated to your app. Usually wrapped within -----BEGIN PRIVATE KEY----- X -----END PRIVATE KEY-----.

### teamId

```ts
teamId: string
```

TeamId is a 10-character string associated with your developer account (usually could be found next to your name in the Apple Developer site).
