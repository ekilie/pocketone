Source: https://pocketbase.io/jsvm/interfaces/security.newJWT.html

# Interface newJWT

**Hierarchy**

- newJWT

```ts
newJWT(payload, signingKey, duration): string
```

NewJWT generates and returns new HS256 signed JWT.

**Parameters**

- `payload: MapClaims`
- `signingKey: string`
- `duration: Duration`

**Returns string**
