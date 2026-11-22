Source: https://pocketbase.io/jsvm/interfaces/dbx.notIn.html

# Interface notIn

**Hierarchy**

- notIn

```ts
notIn(col, ...values): Expression
```

NotIn generates an NOT IN expression for the specified column and the list of disallowed values. If values is empty, an empty string will be returned indicating a true expression.

**Parameters**

- `col: string`
- `Rest ...values: {}[]`

**Returns Expression**
