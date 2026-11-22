Source: https://pocketbase.io/jsvm/interfaces/dbx.like.html

# Interface like

**Hierarchy**

- like

```ts
like(col, ...values): LikeExp
```

Like generates a LIKE expression for the specified column and the possible strings that the column should be like. If multiple values are present, the column should be like *all* of them. For example, Like("name", "key", "word") will generate a SQL expression: "name" LIKE "%key%" AND "name" LIKE "%word%".

By default, each value will be surrounded by "%" to enable partial matching. If a value contains special characters such as "%", "", "_", they will also be properly escaped.

You may call Escape() and/or Match() to change the default behavior. For example, Like("name", "key").Match(false, true) generates "name" LIKE "key%".

**Parameters**

- `col: string`
- `Rest ...values: string[]`

**Returns LikeExp**
