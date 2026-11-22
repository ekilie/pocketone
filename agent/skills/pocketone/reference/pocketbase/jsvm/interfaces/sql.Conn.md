Source: https://pocketbase.io/jsvm/interfaces/sql.Conn.html

# Interface Conn

Conn represents a single database connection rather than a pool of database connections. Prefer running queries from [DB] unless there is a specific need for a continuous single database connection.

A Conn must call [Conn.Close] to return the connection to the database pool and may do so concurrently with a running query.

After a call to [Conn.Close], all operations on the connection fail with [ErrConnDone].

**Hierarchy**

- Conn

## Methods

### beginTx

```ts
beginTx(ctx, opts): sql.Tx
```

BeginTx starts a transaction.

The provided context is used until the transaction is committed or rolled back. If the context is canceled, the sql package will roll back the transaction. [Tx.Commit] will return an error if the context provided to BeginTx is canceled.

The provided [TxOptions] is optional and may be nil if defaults should be used. If a non-default isolation level is used that the driver doesn't support, an error will be returned.

**Parameters**

- `ctx: context.Context`
- `opts: TxOptions`

**Returns sql.Tx**

### close

```ts
close(): void
```

Close returns the connection to the connection pool. All operations after a Close will return with [ErrConnDone]. Close is safe to call concurrently with other operations and will block until all other operations finish. It may be useful to first cancel any used context and then call close directly after.

**Returns void**

### execContext

```ts
execContext(ctx, query, ...args): sql.Result
```

ExecContext executes a query without returning any rows. The args are for any placeholder parameters in the query.

**Parameters**

- `ctx: context.Context`
- `query: string`
- `Rest ...args: any[]`

**Returns sql.Result**

### pingContext

```ts
pingContext(ctx): void
```

PingContext verifies the connection to the database is still alive.

**Parameters**

- `ctx: context.Context`

**Returns void**

### prepareContext

```ts
prepareContext(ctx, query): Stmt
```

PrepareContext creates a prepared statement for later queries or executions. Multiple queries or executions may be run concurrently from the returned statement. The caller must call the statement's [*Stmt.Close] method when the statement is no longer needed.

The provided context is used for the preparation of the statement, not for the execution of the statement.

**Parameters**

- `ctx: context.Context`
- `query: string`

**Returns Stmt**

### queryContext

```ts
queryContext(ctx, query, ...args): sql.Rows
```

QueryContext executes a query that returns rows, typically a SELECT. The args are for any placeholder parameters in the query.

**Parameters**

- `ctx: context.Context`
- `query: string`
- `Rest ...args: any[]`

**Returns sql.Rows**

### queryRowContext

```ts
queryRowContext(ctx, query, ...args): Row
```

QueryRowContext executes a query that is expected to return at most one row. QueryRowContext always returns a non-nil value. Errors are deferred until the [*Row.Scan] method is called. If the query selects no rows, the [*Row.Scan] will return [ErrNoRows]. Otherwise, the [*Row.Scan] scans the first selected row and discards the rest.

**Parameters**

- `ctx: context.Context`
- `query: string`
- `Rest ...args: any[]`

**Returns Row**

### raw

```ts
raw(f): void
```

Raw executes f exposing the underlying driver connection for the duration of f. The driverConn must not be used outside of f.

Once f returns and err is not [driver.ErrBadConn], the [Conn] will continue to be usable until [Conn.Close] is called.

**Parameters**

- `f: ((driverConn) => void)`

**Returns void**
