Source: https://pocketbase.io/jsvm/interfaces/core.TokenConfig.html

# Interface TokenConfig

**Hierarchy**

- TokenConfig

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

Validate makes TokenConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### duration

```ts
duration: number
```

Duration specifies how long an issued token to be valid (in seconds)

### secret

```ts
secret: string
```
