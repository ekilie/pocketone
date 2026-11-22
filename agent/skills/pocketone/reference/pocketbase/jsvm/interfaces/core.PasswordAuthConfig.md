Source: https://pocketbase.io/jsvm/interfaces/core.PasswordAuthConfig.html

# Interface PasswordAuthConfig

**Hierarchy**

- PasswordAuthConfig

## Methods

### validate

```ts
validate(): void
```

Validate makes PasswordAuthConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### enabled

```ts
enabled: boolean
```

### identityFields

```ts
identityFields: string[]
```

IdentityFields is a list of field names that could be used as identity during password authentication.

Usually only fields that has single column UNIQUE index are accepted as values.
