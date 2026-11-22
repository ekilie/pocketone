Source: https://pocketbase.io/jsvm/interfaces/dbx.ExecHookFunc.html

# Interface ExecHookFunc

ExecHookFunc executes before op allowing custom handling like auto fail/retry.

**Hierarchy**

- ExecHookFunc

```ts
ExecHookFunc(q, op): void
```

**Parameters**

- `q: Query`
- `op: (() => void)`

**Returns void**
