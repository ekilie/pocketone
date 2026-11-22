Source: https://pocketbase.io/jsvm/interfaces/sql.Stmt.html

# Interface Stmt

Stmt is a prepared statement. A Stmt is safe for concurrent use by multiple goroutines.

If a Stmt is prepared on a [Tx] or [Conn], it will be bound to a single underlying connection forever. If the [Tx] or [Conn] closes, the Stmt will become unusable and all operations will return an error. If a Stmt is prepared on a [DB], it will remain usable for the lifetime of the [DB]. When the Stmt needs to execute on a new underlying connection, it will prepare itself on the new connection automatically.

**Hierarchy**

- Stmt

## Methods

### close

```ts
close(): void
```

Close closes the statement.

**Returns void**

### exec

```ts
exec(...args): sql.Result
```

Exec executes a prepared statement with the given arguments and returns a [Result] summarizing the effect of the statement.

Exec uses [context.Background] internally; to specify the context, use [Stmt.ExecContext].

**Parameters**

- `Rest ...args: any[]`

**Returns sql.Result**

### execContext

```ts
execContext(ctx, ...args): sql.Result
```

ExecContext executes a prepared statement with the given arguments and returns a [Result] summarizing the effect of the statement.

**Parameters**

- `ctx: context.Context`
- `Rest ...args: any[]`

**Returns sql.Result**

### query

```ts
query(...args): sql.Rows
```

Query executes a prepared query statement with the given arguments and returns the query results as a *Rows.

Query uses [context.Background] internally; to specify the context, use [Stmt.QueryContext].

**Parameters**

- `Rest ...args: any[]`

**Returns sql.Rows**

### queryContext

```ts
queryContext(ctx, ...args): sql.Rows
```

QueryContext executes a prepared query statement with the given arguments and returns the query results as a [*Rows].

**Parameters**

- `ctx: context.Context`
- `Rest ...args: any[]`

**Returns sql.Rows**

### queryRow

```ts
queryRow(...args): Row
```

QueryRow executes a prepared query statement with the given arguments. If an error occurs during the execution of the statement, that error will be returned by a call to Scan on the returned [*Row], which is always non-nil. If the query selects no rows, the [*Row.Scan] will return [ErrNoRows]. Otherwise, the [*Row.Scan] scans the first selected row and discards the rest.

Example usage:

```
 var name string
 err := nameByUseridStmt.QueryRow(id).Scan(&name)
```

QueryRow uses [context.Background] internally; to specify the context, use [Stmt.QueryRowContext].

**Parameters**

- `Rest ...args: any[]`

**Returns Row**

### queryRowContext

```ts
queryRowContext(ctx, ...args): Row
```

QueryRowContext executes a prepared query statement with the given arguments. If an error occurs during the execution of the statement, that error will be returned by a call to Scan on the returned [*Row], which is always non-nil. If the query selects no rows, the [*Row.Scan] will return [ErrNoRows]. Otherwise, the [*Row.Scan] scans the first selected row and discards the rest.

**Parameters**

- `ctx: context.Context`
- `Rest ...args: any[]`

**Returns Row**
