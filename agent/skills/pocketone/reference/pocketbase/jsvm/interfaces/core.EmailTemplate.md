Source: https://pocketbase.io/jsvm/interfaces/core.EmailTemplate.html

# Interface EmailTemplate

**Hierarchy**

- EmailTemplate

## Methods

### resolve

```ts
resolve(placeholders): [string, string]
```

Resolve replaces the placeholder parameters in the current email template and returns its components as ready-to-use strings.

**Parameters**

- `placeholders: _TygojaDict`

**Returns [string, string]**

### validate

```ts
validate(): void
```

Validate makes EmailTemplate validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### body

```ts
body: string
```

### subject

```ts
subject: string
```
