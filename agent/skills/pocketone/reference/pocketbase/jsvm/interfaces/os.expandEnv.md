Source: https://pocketbase.io/jsvm/interfaces/os.expandEnv.html

# Interface expandEnv

**Hierarchy**

- expandEnv

```ts
expandEnv(s): string
```

ExpandEnv replaces ${var} or $var in the string according to the values of the current environment variables. References to undefined variables are replaced by the empty string.

**Parameters**

- `s: string`

**Returns string**
