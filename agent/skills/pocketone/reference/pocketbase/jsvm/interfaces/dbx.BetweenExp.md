Source: https://pocketbase.io/jsvm/interfaces/dbx.BetweenExp.html

# Interface BetweenExp

BetweenExp represents a BETWEEN or a NOT BETWEEN expression.

**Hierarchy**

- BetweenExp

## Methods

### build

```ts
build(db, params): string
```

Build converts an expression into a SQL fragment.

**Parameters**

- `db: dbx.DB`
- `params: Params`

**Returns string**
