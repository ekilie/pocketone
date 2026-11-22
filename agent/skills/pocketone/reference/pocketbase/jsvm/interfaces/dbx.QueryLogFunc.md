Source: https://pocketbase.io/jsvm/interfaces/dbx.QueryLogFunc.html

# Interface QueryLogFunc

QueryLogFunc is called each time when performing a SQL query. The "t" parameter gives the time that the SQL statement takes to execute, while rows and err are the result of the query.

**Hierarchy**

- QueryLogFunc

```ts
QueryLogFunc(ctx, t, sql, rows, err): void
```

**Parameters**

- `ctx: context.Context`
- `t: Duration`
- `sql: string`
- `rows: sql.Rows`
- `err: Error`

**Returns void**
