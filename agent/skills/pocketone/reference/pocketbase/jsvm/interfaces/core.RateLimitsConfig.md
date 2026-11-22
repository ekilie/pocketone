Source: https://pocketbase.io/jsvm/interfaces/core.RateLimitsConfig.html

# Interface RateLimitsConfig

**Hierarchy**

- RateLimitsConfig

## Methods

### findRateLimitRule

```ts
findRateLimitRule(searchLabels, ...optOnlyAudience): [RateLimitRule, boolean]
```

FindRateLimitRule returns the first matching rule based on the provided labels.

Optionally you can further specify a list of valid RateLimitRule.Audience values to further filter the matching rule (aka. the rule Audience will have to exist in one of the specified options).

**Parameters**

- `searchLabels: string[]`
- `Rest ...optOnlyAudience: string[]`

**Returns [RateLimitRule, boolean]**

### validate

```ts
validate(): void
```

Validate makes RateLimitsConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### enabled

```ts
enabled: boolean
```

### excludedIPs

```ts
excludedIPs: string[]
```

### rules

```ts
rules: RateLimitRule[]
```
