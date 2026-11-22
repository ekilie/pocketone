Source: https://pocketbase.io/jsvm/interfaces/dbx.QueryBuilder.html

# Interface QueryBuilder

QueryBuilder builds different clauses for a SELECT SQL statement.

**Hierarchy**

- QueryBuilder

**Indexable**

```ts
[key: string]: any
```

## Methods

### buildFrom

```ts
buildFrom(tables): string
```

BuildFrom generates a FROM clause from the given tables.

**Parameters**

- `tables: string[]`

**Returns string**

### buildGroupBy

```ts
buildGroupBy(cols): string
```

BuildGroupBy generates a GROUP BY clause from the given group-by columns.

**Parameters**

- `cols: string[]`

**Returns string**

### buildHaving

```ts
buildHaving(_arg0, _arg1): string
```

BuildHaving generates a HAVING clause from the given expression.

**Parameters**

- `_arg0: Expression`
- `_arg1: Params`

**Returns string**

### buildJoin

```ts
buildJoin(_arg0, _arg1): string
```

BuildJoin generates a JOIN clause from the given join information.

**Parameters**

- `_arg0: JoinInfo[]`
- `_arg1: Params`

**Returns string**

### buildOrderByAndLimit

```ts
buildOrderByAndLimit(_arg0, _arg1, _arg2, _arg3): string
```

BuildOrderByAndLimit generates the ORDER BY and LIMIT clauses.

**Parameters**

- `_arg0: string`
- `_arg1: string[]`
- `_arg2: number`
- `_arg3: number`

**Returns string**

### buildSelect

```ts
buildSelect(cols, distinct, option): string
```

BuildSelect generates a SELECT clause from the given selected column names.

**Parameters**

- `cols: string[]`
- `distinct: boolean`
- `option: string`

**Returns string**

### buildUnion

```ts
buildUnion(_arg0, _arg1): string
```

BuildUnion generates a UNION clause from the given union information.

**Parameters**

- `_arg0: UnionInfo[]`
- `_arg1: Params`

**Returns string**

### buildWhere

```ts
buildWhere(_arg0, _arg1): string
```

BuildWhere generates a WHERE clause from the given expression.

**Parameters**

- `_arg0: Expression`
- `_arg1: Params`

**Returns string**

### combineUnion

```ts
combineUnion(sql, unionClause): string
```

CombineUnion combines the nonempty unionClause with the provided sql string.

The unionClause is expected to be the result of BuildUnion. If the unionClause is an empty string it returns the sql argument unmodified.

This method exists as a workaround to minimize breaking changes and to allow different SQL builders to specify for example whether they support parenthesis around the UNION SQL queries (SQLite for example will throw a SyntaxError if the UNION parts are wrapped in parenthesis).

**Parameters**

- `sql: string`
- `unionClause: string`

**Returns string**
