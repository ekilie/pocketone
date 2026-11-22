Source: https://pocketbase.io/jsvm/interfaces/dbx.AndOrExp.html

# Interface AndOrExp

AndOrExp represents an expression that concatenates multiple expressions using either "AND" or "OR".

**Hierarchy**

- AndOrExp

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
