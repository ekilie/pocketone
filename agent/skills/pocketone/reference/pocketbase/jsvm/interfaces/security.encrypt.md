Source: https://pocketbase.io/jsvm/interfaces/security.encrypt.html

# Interface encrypt

**Hierarchy**

- encrypt

```ts
encrypt(data, key): string
```

Encrypt encrypts "data" with the specified "key" (must be valid 32 char AES key).

This method uses AES-256-GCM block cypher mode.

**Parameters**

- `data: string | number[]`
- `key: string`

**Returns string**
