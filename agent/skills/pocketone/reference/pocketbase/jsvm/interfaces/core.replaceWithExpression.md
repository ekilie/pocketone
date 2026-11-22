Source: https://pocketbase.io/jsvm/interfaces/core.replaceWithExpression.html

# Interface replaceWithExpression

replaceWithExpression defines a custom expression that will replace a placeholder identifier found in "old" with the result of "new".

**Hierarchy**

- replaceWithExpression

## Methods

### build

```ts
build(db, params): string
```

Build converts the expression into a SQL fragment.

Implements [dbx.Expression] interface.

**Parameters**

- `db: dbx.DB`
- `params: Params`

**Returns string**
