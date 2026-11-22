Source: https://pocketbase.io/jsvm/interfaces/dbx.HashExp.html

# Interface HashExp

HashExp represents a hash expression.

A hash expression is a map whose keys are DB column names which need to be filtered according to the corresponding values. For example, HashExp{"level": 2, "dept": 10} will generate the SQL: "level"=2 AND "dept"=10.

HashExp also handles nil values and slice values. For example, HashExp{"level": []interface{}{1, 2}, "dept": nil} will generate: "level" IN (1, 2) AND "dept" IS NULL.

**Hierarchy**

- [_TygojaDict](https://pocketbase.io/jsvm/types/_TygojaDict.html)

  - HashExp

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
