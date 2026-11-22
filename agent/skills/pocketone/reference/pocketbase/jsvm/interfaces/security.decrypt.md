Source: https://pocketbase.io/jsvm/interfaces/security.decrypt.html

# Interface decrypt

**Hierarchy**

- decrypt

```ts
decrypt(cipherText, key): string | number[]
```

Decrypt decrypts encrypted text with key (must be valid 32 chars AES key).

This method uses AES-256-GCM block cypher mode.

**Parameters**

- `cipherText: string`
- `key: string`

**Returns string | number[]**
