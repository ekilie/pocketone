Source: https://pocketbase.io/jsvm/interfaces/security.randomStringWithAlphabet.html

# Interface randomStringWithAlphabet

**Hierarchy**

- randomStringWithAlphabet

```ts
randomStringWithAlphabet(length, alphabet): string
```

RandomStringWithAlphabet generates a cryptographically random string with the specified length and characters set.

It panics if for some reason rand.Int returns a non-nil error.

**Parameters**

- `length: number`
- `alphabet: string`

**Returns string**
