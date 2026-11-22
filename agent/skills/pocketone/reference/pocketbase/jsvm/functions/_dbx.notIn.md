Source: https://pocketbase.io/jsvm/functions/_dbx.notIn.html

# Function notIn

```ts
notIn(col, ...values): Expression
```

NotIn generates an NOT IN expression for the specified column and the list of disallowed values. If values is empty, an empty string will be returned indicating a true expression.

**Parameters**

- `col: string`
- `Rest ...values: {}[]`

**Returns Expression**
