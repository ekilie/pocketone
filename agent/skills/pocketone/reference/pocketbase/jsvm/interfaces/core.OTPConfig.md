Source: https://pocketbase.io/jsvm/interfaces/core.OTPConfig.html

# Interface OTPConfig

**Hierarchy**

- OTPConfig

## Methods

### durationTime

```ts
durationTime(): Duration
```

DurationTime returns the current Duration as [time.Duration].

**Returns Duration**

### validate

```ts
validate(): void
```

Validate makes OTPConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### duration

```ts
duration: number
```

Duration specifies how long the OTP to be valid (in seconds)

### emailTemplate

```ts
emailTemplate: EmailTemplate
```

EmailTemplate is the default OTP email template that will be send to the auth record.

In addition to the system placeholders you can also make use of [core.EmailPlaceholderOTPId] and [core.EmailPlaceholderOTP].

### enabled

```ts
enabled: boolean
```

### length

```ts
length: number
```

Length specifies the auto generated password length.
