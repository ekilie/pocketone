Source: https://pocketbase.io/jsvm/interfaces/dbx.Executor.html

# Interface Executor

Executor prepares, executes, or queries a SQL statement.

**Hierarchy**

- Executor

**Indexable**

```ts
[key: string]: any
```

## Methods

### exec

```ts
exec(query, ...args): sql.Result
```

Exec executes a SQL statement

**Parameters**

- `query: string`
- `Rest ...args: {}[]`

**Returns sql.Result**

### execContext

```ts
execContext(ctx, query, ...args): sql.Result
```

ExecContext executes a SQL statement with the given context

**Parameters**

- `ctx: context.Context`
- `query: string`
- `Rest ...args: {}[]`

**Returns sql.Result**

### prepare

```ts
prepare(query): Stmt
```

Prepare creates a prepared statement

**Parameters**

- `query: string`

**Returns Stmt**

### query

```ts
query(query, ...args): sql.Rows
```

Query queries a SQL statement

**Parameters**

- `query: string`
- `Rest ...args: {}[]`

**Returns sql.Rows**

### queryContext

```ts
queryContext(ctx, query, ...args): sql.Rows
```

QueryContext queries a SQL statement with the given context

**Parameters**

- `ctx: context.Context`
- `query: string`
- `Rest ...args: {}[]`

**Returns sql.Rows**
