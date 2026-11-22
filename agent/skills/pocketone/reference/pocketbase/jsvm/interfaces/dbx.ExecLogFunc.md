Source: https://pocketbase.io/jsvm/interfaces/dbx.ExecLogFunc.html

# Interface ExecLogFunc

ExecLogFunc is called each time when a SQL statement is executed. The "t" parameter gives the time that the SQL statement takes to execute, while result and err refer to the result of the execution.

**Hierarchy**

- ExecLogFunc

```ts
ExecLogFunc(ctx, t, sql, result, err): void
```

**Parameters**

- `ctx: context.Context`
- `t: Duration`
- `sql: string`
- `result: sql.Result`
- `err: Error`

**Returns void**
