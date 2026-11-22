Source: https://pocketbase.io/jsvm/interfaces/jwt.ClaimStrings.html

# Interface ClaimStrings

ClaimStrings is basically just a slice of strings, but it can be either serialized from a string array or just a string. This type is necessary, since the "aud" claim can either be a single string or an array.

**Hierarchy**

- Array<string>

  - ClaimStrings

## Methods

### marshalJSON

```ts
marshalJSON(): string | number[]
```

**Returns string | number[]**

### unmarshalJSON

```ts
unmarshalJSON(data): void
```

**Parameters**

- `data: string | number[]`

**Returns void**
