Source: https://pocketbase.io/jsvm/interfaces/dbx._in.html

# Interface _in

**Hierarchy**

- _in

```ts
_in(col, ...values): Expression
```

In generates an IN expression for the specified column and the list of allowed values. If values is empty, a SQL "0=1" will be generated which represents a false expression.

**Parameters**

- `col: string`
- `Rest ...values: {}[]`

**Returns Expression**
