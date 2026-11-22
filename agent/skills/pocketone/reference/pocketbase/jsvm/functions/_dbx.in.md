Source: https://pocketbase.io/jsvm/functions/_dbx.in.html

# Function in

```ts
in(col, ...values): Expression
```

In generates an IN expression for the specified column and the list of allowed values. If values is empty, a SQL "0=1" will be generated which represents a false expression.

**Parameters**

- `col: string`
- `Rest ...values: {}[]`

**Returns Expression**
