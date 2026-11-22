Source: https://pocketbase.io/jsvm/interfaces/core.collectionAuthOptions.html

# Interface collectionAuthOptions

collectionAuthOptions defines the options for the "auth" type collection.

**Hierarchy**

- collectionAuthOptions

## Properties

### authAlert

```ts
authAlert: AuthAlertConfig
```

AuthAlert defines options related to the auth alerts on new device login.

### Optional authRule

```ts
authRule?: string
```

AuthRule could be used to specify additional record constraints applied after record authentication and right before returning the auth token response to the client.

For example, to allow only verified users you could set it to "verified = true".

Set it to empty string to allow any Auth collection record to authenticate.

Set it to nil to disallow authentication altogether for the collection (that includes password, OAuth2, etc.).

### authToken

```ts
authToken: TokenConfig
```

## Various token configurations

### confirmEmailChangeTemplate

```ts
confirmEmailChangeTemplate: EmailTemplate
```

### emailChangeToken

```ts
emailChangeToken: TokenConfig
```

### fileToken

```ts
fileToken: TokenConfig
```

### Optional manageRule

```ts
manageRule?: string
```

ManageRule gives admin-like permissions to allow fully managing the auth record(s), eg. changing the password without requiring to enter the old one, directly updating the verified state and email, etc.

This rule is executed in addition to the Create and Update API rules.

### mfa

```ts
mfa: MFAConfig
```

MFA defines options related to the Multi-factor authentication (MFA).

### oauth2

```ts
oauth2: OAuth2Config
```

OAuth2 specifies whether OAuth2 auth is enabled for the collection and which OAuth2 providers are allowed.

### otp

```ts
otp: OTPConfig
```

OTP defines options related to the One-time password authentication (OTP).

### passwordAuth

```ts
passwordAuth: PasswordAuthConfig
```

PasswordAuth defines options related to the collection password authentication.

### passwordResetToken

```ts
passwordResetToken: TokenConfig
```

### resetPasswordTemplate

```ts
resetPasswordTemplate: EmailTemplate
```

### verificationTemplate

```ts
verificationTemplate: EmailTemplate
```

## Default email templates

### verificationToken

```ts
verificationToken: TokenConfig
```
