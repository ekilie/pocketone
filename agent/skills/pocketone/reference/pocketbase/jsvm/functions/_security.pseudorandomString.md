Source: https://pocketbase.io/jsvm/functions/_security.pseudorandomString.html

# Function pseudorandomString

```ts
pseudorandomString(length): string
```

PseudorandomString generates a pseudorandom string with the specified length.

The generated string matches [A-Za-z0-9]+ and it's transparent to URL-encoding.

For a cryptographically random string (but a little bit slower) use RandomString instead.

**Parameters**

- `length: number`

**Returns string**
