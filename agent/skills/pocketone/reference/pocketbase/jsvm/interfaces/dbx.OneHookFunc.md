Source: https://pocketbase.io/jsvm/interfaces/dbx.OneHookFunc.html

# Interface OneHookFunc

OneHookFunc executes right before the query populate the row result from One() call (aka. op).

**Hierarchy**

- OneHookFunc

```ts
OneHookFunc(q, a, op): void
```

**Parameters**

- `q: Query`
- `a: {}`
- `op: ((b) => void)`

**Returns void**
