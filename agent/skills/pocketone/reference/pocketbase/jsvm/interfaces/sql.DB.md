Source: https://pocketbase.io/jsvm/interfaces/sql.DB.html

# Interface DB

DB is a database handle representing a pool of zero or more underlying connections. It's safe for concurrent use by multiple goroutines.

The sql package creates and frees connections automatically; it also maintains a free pool of idle connections. If the database has a concept of per-connection state, such state can be reliably observed within a transaction ([Tx]) or connection ([Conn]). Once [DB.Begin] is called, the returned [Tx] is bound to a single connection. Once [Tx.Commit] or [Tx.Rollback] is called on the transaction, that transaction's connection is returned to [DB]'s idle connection pool. The pool size can be controlled with [DB.SetMaxIdleConns].

**Hierarchy**

- DB

## Methods

### begin

```ts
begin(): sql.Tx
```

Begin starts a transaction. The default isolation level is dependent on the driver.

Begin uses [context.Background] internally; to specify the context, use [DB.BeginTx].

**Returns sql.Tx**

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

Close closes the database and prevents new queries from starting. Close then waits for all queries that have started processing on the server to finish.

It is rare to Close a [DB], as the [DB] handle is meant to be long-lived and shared between many goroutines.

**Returns void**

### conn

```ts
conn(ctx): sql.Conn
```

Conn returns a single connection by either opening a new connection or returning an existing connection from the connection pool. Conn will block until either a connection is returned or ctx is canceled. Queries run on the same Conn will be run in the same database session.

Every Conn must be returned to the database pool after use by calling [Conn.Close].

**Parameters**

- `ctx: context.Context`

**Returns sql.Conn**

### driver

```ts
driver(): any
```

Driver returns the database's underlying driver.

**Returns any**

### exec

```ts
exec(query, ...args): sql.Result
```

Exec executes a query without returning any rows. The args are for any placeholder parameters in the query.

Exec uses [context.Background] internally; to specify the context, use [DB.ExecContext].

**Parameters**

- `query: string`
- `Rest ...args: any[]`

**Returns sql.Result**

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

### ping

```ts
ping(): void
```

Ping verifies a connection to the database is still alive, establishing a connection if necessary.

Ping uses [context.Background] internally; to specify the context, use [DB.PingContext].

**Returns void**

### pingContext

```ts
pingContext(ctx): void
```

PingContext verifies a connection to the database is still alive, establishing a connection if necessary.

**Parameters**

- `ctx: context.Context`

**Returns void**

### prepare

```ts
prepare(query): Stmt
```

Prepare creates a prepared statement for later queries or executions. Multiple queries or executions may be run concurrently from the returned statement. The caller must call the statement's [*Stmt.Close] method when the statement is no longer needed.

Prepare uses [context.Background] internally; to specify the context, use [DB.PrepareContext].

**Parameters**

- `query: string`

**Returns Stmt**

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

### query

```ts
query(query, ...args): sql.Rows
```

Query executes a query that returns rows, typically a SELECT. The args are for any placeholder parameters in the query.

Query uses [context.Background] internally; to specify the context, use [DB.QueryContext].

**Parameters**

- `query: string`
- `Rest ...args: any[]`

**Returns sql.Rows**

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

### queryRow

```ts
queryRow(query, ...args): Row
```

QueryRow executes a query that is expected to return at most one row. QueryRow always returns a non-nil value. Errors are deferred until [Row]'s Scan method is called. If the query selects no rows, the [*Row.Scan] will return [ErrNoRows]. Otherwise, [*Row.Scan] scans the first selected row and discards the rest.

QueryRow uses [context.Background] internally; to specify the context, use [DB.QueryRowContext].

**Parameters**

- `query: string`
- `Rest ...args: any[]`

**Returns Row**

### queryRowContext

```ts
queryRowContext(ctx, query, ...args): Row
```

QueryRowContext executes a query that is expected to return at most one row. QueryRowContext always returns a non-nil value. Errors are deferred until [Row]'s Scan method is called. If the query selects no rows, the [*Row.Scan] will return [ErrNoRows]. Otherwise, [*Row.Scan] scans the first selected row and discards the rest.

**Parameters**

- `ctx: context.Context`
- `query: string`
- `Rest ...args: any[]`

**Returns Row**

### setConnMaxIdleTime

```ts
setConnMaxIdleTime(d): void
```

SetConnMaxIdleTime sets the maximum amount of time a connection may be idle.

Expired connections may be closed lazily before reuse.

If d <= 0, connections are not closed due to a connection's idle time.

**Parameters**

- `d: Duration`

**Returns void**

### setConnMaxLifetime

```ts
setConnMaxLifetime(d): void
```

SetConnMaxLifetime sets the maximum amount of time a connection may be reused.

Expired connections may be closed lazily before reuse.

If d <= 0, connections are not closed due to a connection's age.

**Parameters**

- `d: Duration`

**Returns void**

### setMaxIdleConns

```ts
setMaxIdleConns(n): void
```

SetMaxIdleConns sets the maximum number of connections in the idle connection pool.

If MaxOpenConns is greater than 0 but less than the new MaxIdleConns, then the new MaxIdleConns will be reduced to match the MaxOpenConns limit.

If n <= 0, no idle connections are retained.

The default max idle connections is currently 2. This may change in a future release.

**Parameters**

- `n: number`

**Returns void**

### setMaxOpenConns

```ts
setMaxOpenConns(n): void
```

SetMaxOpenConns sets the maximum number of open connections to the database.

If MaxIdleConns is greater than 0 and the new MaxOpenConns is less than MaxIdleConns, then MaxIdleConns will be reduced to match the new MaxOpenConns limit.

If n <= 0, then there is no limit on the number of open connections. The default is 0 (unlimited).

**Parameters**

- `n: number`

**Returns void**

### stats

```ts
stats(): DBStats
```

Stats returns database statistics.

**Returns DBStats**
