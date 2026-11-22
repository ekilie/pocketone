Source: https://pocketbase.io/jsvm/interfaces/os.expand.html

# Interface expand

**Hierarchy**

- expand

```ts
expand(s, mapping): string
```

Expand replaces ${var} or $var in the string based on the mapping function. For example, [os.ExpandEnv](https://pocketbase.io/jsvm/interfaces/s) is equivalent to [os.Expand](s, [os.Getenv]).

**Parameters**

- `s: string`
- `mapping: ((_arg0) => string)`

**Returns string**
