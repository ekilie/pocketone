Source: https://pocketbase.io/jsvm/interfaces/dbx.NotExp.html

# Interface NotExp

NotExp represents an expression that should prefix "NOT" to a specified expression.

**Hierarchy**

- NotExp

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
