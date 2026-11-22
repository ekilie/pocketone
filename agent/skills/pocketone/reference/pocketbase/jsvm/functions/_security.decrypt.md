Source: https://pocketbase.io/jsvm/functions/_security.decrypt.html

# Function decrypt

```ts
decrypt(cipherText, key): string | number[]
```

Decrypt decrypts encrypted text with key (must be valid 32 chars AES key).

This method uses AES-256-GCM block cypher mode.

**Parameters**

- `cipherText: string`
- `key: string`

**Returns string | number[]**
