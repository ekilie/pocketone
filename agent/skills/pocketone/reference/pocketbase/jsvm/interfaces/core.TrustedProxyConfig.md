Source: https://pocketbase.io/jsvm/interfaces/core.TrustedProxyConfig.html

# Interface TrustedProxyConfig

**Hierarchy**

- TrustedProxyConfig

## Methods

### validate

```ts
validate(): void
```

Validate makes RateLimitRule validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### headers

```ts
headers: string[]
```

Headers is a list of explicit trusted header(s) to check.

### useLeftmostIP

```ts
useLeftmostIP: boolean
```

UseLeftmostIP specifies to use the left-mostish IP from the trusted headers.

Note that this could be insecure when used with X-Forwarded-For header because some proxies like AWS ELB allow users to prepend their own header value before appending the trusted ones.
