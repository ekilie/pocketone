Source: https://pocketbase.io/jsvm/functions/_dbx.orLike.html

# Function orLike

```ts
orLike(col, ...values): LikeExp
```

OrLike generates an OR LIKE expression. This is similar to Like() except that the column should be like one of the possible values. For example, OrLike("name", "key", "word") will generate a SQL expression: "name" LIKE "%key%" OR "name" LIKE "%word%". Please see Like() for more details.

**Parameters**

- `col: string`
- `Rest ...values: string[]`

**Returns LikeExp**
