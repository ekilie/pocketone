Source: https://pocketbase.io/jsvm/functions/_dbx.notLike.html

# Function notLike

```ts
notLike(col, ...values): LikeExp
```

NotLike generates a NOT LIKE expression. For example, NotLike("name", "key", "word") will generate a SQL expression: "name" NOT LIKE "%key%" AND "name" NOT LIKE "%word%". Please see Like() for more details.

**Parameters**

- `col: string`
- `Rest ...values: string[]`

**Returns LikeExp**
