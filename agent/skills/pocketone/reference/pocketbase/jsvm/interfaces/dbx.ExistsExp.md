Source: https://pocketbase.io/jsvm/interfaces/dbx.ExistsExp.html

# Interface ExistsExp

ExistsExp represents an EXISTS or NOT EXISTS expression.

**Hierarchy**

- ExistsExp

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
