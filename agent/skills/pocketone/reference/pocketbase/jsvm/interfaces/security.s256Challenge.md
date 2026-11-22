Source: https://pocketbase.io/jsvm/interfaces/security.s256Challenge.html

# Interface s256Challenge

**Hierarchy**

- s256Challenge

```ts
s256Challenge(code): string
```

S256Challenge creates base64 encoded sha256 challenge string derived from code. The padding of the result base64 string is stripped per [RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636#section-4.2).

**Parameters**

- `code: string`

**Returns string**
