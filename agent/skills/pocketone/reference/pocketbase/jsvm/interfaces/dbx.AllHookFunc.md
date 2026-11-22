Source: https://pocketbase.io/jsvm/interfaces/dbx.AllHookFunc.html

# Interface AllHookFunc

AllHookFunc executes right before the query populate the row result from All() call (aka. op).

**Hierarchy**

- AllHookFunc

```ts
AllHookFunc(q, sliceA, op): void
```

**Parameters**

- `q: Query`
- `sliceA: {}`
- `op: ((sliceB) => void)`

**Returns void**
