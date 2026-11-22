Source: https://pocketbase.io/jsvm/interfaces/dbx.SelectQuery.html

# Interface SelectQuery

SelectQuery represents a DB-agnostic SELECT query. It can be built into a DB-specific query by calling the Build() method.

**Hierarchy**

- SelectQuery

## Methods

### all

```ts
all(slice): void
```

All executes the SELECT query and populates all rows of the result into a slice.

Note that the slice must be passed in as a pointer.

If the query does not specify a "from" clause, the method will try to infer the name of the table to be selected from by calling getTableName() which will return either the type name of the slice elements or the TableName() method if the slice element implements the TableModel interface.

**Parameters**

- `slice: {}`

**Returns void**

### andBind

```ts
andBind(params): SelectQuery
```

AndBind appends additional parameters to be bound to the query.

**Parameters**

- `params: Params`

**Returns SelectQuery**

### andGroupBy

```ts
andGroupBy(...cols): SelectQuery
```

AndGroupBy appends additional columns to the existing GROUP BY clause. Column names will be properly quoted.

**Parameters**

- `Rest ...cols: string[]`

**Returns SelectQuery**

### andHaving

```ts
andHaving(e): SelectQuery
```

AndHaving concatenates a new HAVING condition with the existing one (if any) using "AND".

**Parameters**

- `e: Expression`

**Returns SelectQuery**

### andOrderBy

```ts
andOrderBy(...cols): SelectQuery
```

AndOrderBy appends additional columns to the existing ORDER BY clause. Column names will be properly quoted. A column name can contain "ASC" or "DESC" to indicate its ordering direction.

**Parameters**

- `Rest ...cols: string[]`

**Returns SelectQuery**

### andSelect

```ts
andSelect(...cols): SelectQuery
```

AndSelect adds additional columns to be selected. Column names will be automatically quoted.

**Parameters**

- `Rest ...cols: string[]`

**Returns SelectQuery**

### andWhere

```ts
andWhere(e): SelectQuery
```

AndWhere concatenates a new WHERE condition with the existing one (if any) using "AND".

**Parameters**

- `e: Expression`

**Returns SelectQuery**

### bind

```ts
bind(params): SelectQuery
```

Bind specifies the parameter values to be bound to the query.

**Parameters**

- `params: Params`

**Returns SelectQuery**

### build

```ts
build(): Query
```

Build builds the SELECT query and returns an executable Query object.

**Returns Query**

### column

```ts
column(a): void
```

Column builds and executes the SELECT statement and populates the first column of the result into a slice. Note that the parameter must be a pointer to a slice. This is a shortcut to SelectQuery.Build().Column()

**Parameters**

- `a: {}`

**Returns void**

### context

```ts
context(): context.Context
```

Context returns the context associated with the query.

**Returns context.Context**

### distinct

```ts
distinct(v): SelectQuery
```

Distinct specifies whether to select columns distinctively. By default, distinct is false.

**Parameters**

- `v: boolean`

**Returns SelectQuery**

### from

```ts
from(...tables): SelectQuery
```

From specifies which tables to select from. Table names will be automatically quoted.

**Parameters**

- `Rest ...tables: string[]`

**Returns SelectQuery**

### groupBy

```ts
groupBy(...cols): SelectQuery
```

GroupBy specifies the GROUP BY clause. Column names will be properly quoted.

**Parameters**

- `Rest ...cols: string[]`

**Returns SelectQuery**

### having

```ts
having(e): SelectQuery
```

Having specifies the HAVING clause.

**Parameters**

- `e: Expression`

**Returns SelectQuery**

### info

```ts
info(): QueryInfo
```

Info exports common SelectQuery fields allowing to inspect the current select query options.

**Returns QueryInfo**

### innerJoin

```ts
innerJoin(table, on): SelectQuery
```

InnerJoin specifies an INNER JOIN clause. This is a shortcut method for Join.

**Parameters**

- `table: string`
- `on: Expression`

**Returns SelectQuery**

### join

```ts
join(typ, table, on): SelectQuery
```

Join specifies a JOIN clause. The "typ" parameter specifies the JOIN type (e.g. "INNER JOIN", "LEFT JOIN").

**Parameters**

- `typ: string`
- `table: string`
- `on: Expression`

**Returns SelectQuery**

### leftJoin

```ts
leftJoin(table, on): SelectQuery
```

LeftJoin specifies a LEFT JOIN clause. This is a shortcut method for Join.

**Parameters**

- `table: string`
- `on: Expression`

**Returns SelectQuery**

### limit

```ts
limit(limit): SelectQuery
```

Limit specifies the LIMIT clause. A negative limit means no limit.

**Parameters**

- `limit: number`

**Returns SelectQuery**

### model

```ts
model(pk, model): void
```

Model selects the row with the specified primary key and populates the model with the row data.

The model variable should be a pointer to a struct. If the query does not specify a "from" clause, it will use the model struct to determine which table to select data from. It will also use the model to infer the name of the primary key column. Only simple primary key is supported. For composite primary keys, please use Where() to specify the filtering condition.

**Parameters**

- `pk: {}`
- `model: {}`

**Returns void**

### offset

```ts
offset(offset): SelectQuery
```

Offset specifies the OFFSET clause. A negative offset means no offset.

**Parameters**

- `offset: number`

**Returns SelectQuery**

### one

```ts
one(a): void
```

One executes the SELECT query and populates the first row of the result into the specified variable.

If the query does not specify a "from" clause, the method will try to infer the name of the table to be selected from by calling getTableName() which will return either the variable type name or the TableName() method if the variable implements the TableModel interface.

Note that when the query has no rows in the result set, an sql.ErrNoRows will be returned.

**Parameters**

- `a: {}`

**Returns void**

### orHaving

```ts
orHaving(e): SelectQuery
```

OrHaving concatenates a new HAVING condition with the existing one (if any) using "OR".

**Parameters**

- `e: Expression`

**Returns SelectQuery**

### orWhere

```ts
orWhere(e): SelectQuery
```

OrWhere concatenates a new WHERE condition with the existing one (if any) using "OR".

**Parameters**

- `e: Expression`

**Returns SelectQuery**

### orderBy

```ts
orderBy(...cols): SelectQuery
```

OrderBy specifies the ORDER BY clause. Column names will be properly quoted. A column name can contain "ASC" or "DESC" to indicate its ordering direction.

**Parameters**

- `Rest ...cols: string[]`

**Returns SelectQuery**

### postFragment

```ts
postFragment(fragment): SelectQuery
```

PostFragment sets SQL fragment that should be appended at the end of the main select query.

If there is "UNION"/"UNION ALL" clause then the extra UNION queries are appended after the PostFragment.

**Parameters**

- `fragment: string`

**Returns SelectQuery**

### preFragment

```ts
preFragment(fragment): SelectQuery
```

PreFragment sets SQL fragment that should be prepended before the select query (e.g. WITH clause).

**Parameters**

- `fragment: string`

**Returns SelectQuery**

### rightJoin

```ts
rightJoin(table, on): SelectQuery
```

RightJoin specifies a RIGHT JOIN clause. This is a shortcut method for Join.

**Parameters**

- `table: string`
- `on: Expression`

**Returns SelectQuery**

### row

```ts
row(...a): void
```

Row builds and executes the SELECT query and populates the first row of the result into the specified variables. This is a shortcut to SelectQuery.Build().Row()

**Parameters**

- `Rest ...a: {}[]`

**Returns void**

### rows

```ts
rows(): dbx.Rows
```

Rows builds and executes the SELECT query and returns a Rows object for data retrieval purpose. This is a shortcut to SelectQuery.Build().Rows()

**Returns dbx.Rows**

### select

```ts
select(...cols): SelectQuery
```

Select specifies the columns to be selected. Column names will be automatically quoted.

**Parameters**

- `Rest ...cols: string[]`

**Returns SelectQuery**

### selectOption

```ts
selectOption(option): SelectQuery
```

SelectOption specifies additional option that should be append to "SELECT".

**Parameters**

- `option: string`

**Returns SelectQuery**

### union

```ts
union(q): SelectQuery
```

Union specifies a UNION clause.

**Parameters**

- `q: Query`

**Returns SelectQuery**

### unionAll

```ts
unionAll(q): SelectQuery
```

UnionAll specifies a UNION ALL clause.

**Parameters**

- `q: Query`

**Returns SelectQuery**

### where

```ts
where(e): SelectQuery
```

Where specifies the WHERE condition.

**Parameters**

- `e: Expression`

**Returns SelectQuery**

### withBuildHook

```ts
withBuildHook(fn): SelectQuery
```

WithBuildHook runs the provided hook function with the query created on Build().

**Parameters**

- `fn: BuildHookFunc`

**Returns SelectQuery**

### withContext

```ts
withContext(ctx): SelectQuery
```

WithContext associates a context with the query.

**Parameters**

- `ctx: context.Context`

**Returns SelectQuery**

## Properties

### fieldMapper

```ts
fieldMapper: FieldMapFunc
```

FieldMapper maps struct field names to DB column names.

### tableMapper

```ts
tableMapper: TableMapFunc
```

TableMapper maps structs to DB table names.
