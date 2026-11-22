Source: https://pocketbase.io/jsvm/interfaces/core.MFAConfig.html

# Interface MFAConfig

**Hierarchy**

- MFAConfig

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

Validate makes MFAConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### duration

```ts
duration: number
```

Duration specifies how long an issued MFA to be valid (in seconds)

### enabled

```ts
enabled: boolean
```

### rule

```ts
rule: string
```

Rule is an optional field to restrict MFA only for the records that satisfy the rule.

Leave it empty to enable MFA for everyone.
