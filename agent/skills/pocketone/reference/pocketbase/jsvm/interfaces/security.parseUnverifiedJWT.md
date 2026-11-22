Source: https://pocketbase.io/jsvm/interfaces/security.parseUnverifiedJWT.html

# Interface parseUnverifiedJWT

**Hierarchy**

- parseUnverifiedJWT

```ts
parseUnverifiedJWT(token): MapClaims
```

ParseUnverifiedJWT parses JWT and returns its claims but DOES NOT verify the signature.

It verifies only the exp, iat and nbf claims.

**Parameters**

- `token: string`

**Returns MapClaims**
