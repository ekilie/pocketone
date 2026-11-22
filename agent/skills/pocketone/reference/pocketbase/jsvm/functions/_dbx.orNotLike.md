Source: https://pocketbase.io/jsvm/functions/_dbx.orNotLike.html

# Function orNotLike

```ts
orNotLike(col, ...values): LikeExp
```

OrNotLike generates an OR NOT LIKE expression. For example, OrNotLike("name", "key", "word") will generate a SQL expression: "name" NOT LIKE "%key%" OR "name" NOT LIKE "%word%". Please see Like() for more details.

**Parameters**

- `col: string`
- `Rest ...values: string[]`

**Returns LikeExp**
