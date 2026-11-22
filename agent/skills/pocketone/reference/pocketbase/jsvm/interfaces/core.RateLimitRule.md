Source: https://pocketbase.io/jsvm/interfaces/core.RateLimitRule.html

# Interface RateLimitRule

**Hierarchy**

- RateLimitRule

## Methods

### durationTime

```ts
durationTime(): Duration
```

DurationTime returns the tag's Duration as [time.Duration].

**Returns Duration**

### string

```ts
string(): string
```

String returns a string representation of the rule.

**Returns string**

### validate

```ts
validate(): void
```

Validate makes RateLimitRule validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### audience

```ts
audience: string
```

Audience specifies the auth group the rule should apply for:

```
  - ""      - both guests and authenticated users (default)
  - "@guest" - only for guests
  - "@auth"  - only for authenticated users
```

### duration

```ts
duration: number
```

Duration specifies the interval (in seconds) per which to reset the counted/accumulated rate limiter tokens.

### label

```ts
label: string
```

Label is the identifier of the current rule.

It could be a tag, complete path or path prerefix (when ends with `/`).

Example supported labels:

```
  - test_a (plain text "tag")
  - users:create
  - *:create
  - /
  - /api
  - POST /api/collections/
```

### maxRequests

```ts
maxRequests: number
```

MaxRequests is the max allowed number of requests per Duration.
