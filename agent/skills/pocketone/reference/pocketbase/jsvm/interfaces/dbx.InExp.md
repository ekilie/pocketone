Source: https://pocketbase.io/jsvm/interfaces/dbx.InExp.html

# Interface InExp

InExp represents an "IN" or "NOT IN" expression.

**Hierarchy**

- InExp

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
