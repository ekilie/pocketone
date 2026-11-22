Source: https://pocketbase.io/jsvm/functions/_security.parseUnverifiedJWT.html

# Function parseUnverifiedJWT

```ts
parseUnverifiedJWT(token): _TygojaDict
```

ParseUnverifiedJWT parses JWT and returns its claims but DOES NOT verify the signature.

It verifies only the exp, iat and nbf claims.

**Parameters**

- `token: string`

**Returns _TygojaDict**
