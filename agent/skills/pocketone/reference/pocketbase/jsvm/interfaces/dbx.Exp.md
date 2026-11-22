Source: https://pocketbase.io/jsvm/interfaces/dbx.Exp.html

# Interface Exp

Exp represents an expression with a SQL fragment and a list of optional binding parameters.

**Hierarchy**

- Exp

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
