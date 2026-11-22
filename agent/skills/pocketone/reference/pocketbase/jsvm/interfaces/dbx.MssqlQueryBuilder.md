Source: https://pocketbase.io/jsvm/interfaces/dbx.MssqlQueryBuilder.html

# Interface MssqlQueryBuilder

MssqlQueryBuilder is the query builder for SQL Server databases.

**Hierarchy**

- [_sIZSPhr](https://pocketbase.io/jsvm/types/dbx._sIZSPhr.html)

  - MssqlQueryBuilder

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
buildHaving(e, params): string
```

BuildHaving generates a HAVING clause from the given expression.

**Parameters**

- `e: Expression`
- `params: Params`

**Returns string**

### buildJoin

```ts
buildJoin(joins, params): string
```

BuildJoin generates a JOIN clause from the given join information.

**Parameters**

- `joins: JoinInfo[]`
- `params: Params`

**Returns string**

### buildLimit

```ts
buildLimit(limit, offset): string
```

BuildLimit generates the LIMIT clause.

**Parameters**

- `limit: number`
- `offset: number`

**Returns string**

### buildOrderBy

```ts
buildOrderBy(cols): string
```

BuildOrderBy generates the ORDER BY clause.

**Parameters**

- `cols: string[]`

**Returns string**

### buildOrderByAndLimit

```ts
buildOrderByAndLimit(sql, cols, limit, offset): string
```

BuildOrderByAndLimit generates the ORDER BY and LIMIT clauses.

**Parameters**

- `sql: string`
- `cols: string[]`
- `limit: number`
- `offset: number`

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
buildUnion(unions, params): string
```

BuildUnion generates a UNION clause from the given union information.

**Parameters**

- `unions: UnionInfo[]`
- `params: Params`

**Returns string**

### buildWhere

```ts
buildWhere(e, params): string
```

BuildWhere generates a WHERE clause from the given expression.

**Parameters**

- `e: Expression`
- `params: Params`

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

### db

```ts
db(): dbx.DB
```

DB returns the DB instance associated with the query builder.

**Returns dbx.DB**
