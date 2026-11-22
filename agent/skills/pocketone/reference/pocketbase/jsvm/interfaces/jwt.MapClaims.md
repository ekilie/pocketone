Source: https://pocketbase.io/jsvm/interfaces/jwt.MapClaims.html

# Interface MapClaims

MapClaims is a claims type that uses the map[string]any for JSON decoding. This is the default claims type if you don't supply one

**Hierarchy**

- [_TygojaDict](https://pocketbase.io/jsvm/types/_TygojaDict.html)

  - MapClaims

## Methods

### getAudience

```ts
getAudience(): ClaimStrings
```

GetAudience implements the Claims interface.

**Returns ClaimStrings**

### getExpirationTime

```ts
getExpirationTime(): NumericDate
```

GetExpirationTime implements the Claims interface.

**Returns NumericDate**

### getIssuedAt

```ts
getIssuedAt(): NumericDate
```

GetIssuedAt implements the Claims interface.

**Returns NumericDate**

### getIssuer

```ts
getIssuer(): string
```

GetIssuer implements the Claims interface.

**Returns string**

### getNotBefore

```ts
getNotBefore(): NumericDate
```

GetNotBefore implements the Claims interface.

**Returns NumericDate**

### getSubject

```ts
getSubject(): string
```

GetSubject implements the Claims interface.

**Returns string**
