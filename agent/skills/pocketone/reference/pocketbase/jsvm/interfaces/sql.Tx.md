Source: https://pocketbase.io/jsvm/interfaces/sql.Tx.html

# Interface Tx

Tx is an in-progress database transaction.

A transaction must end with a call to [Tx.Commit] or [Tx.Rollback].

After a call to [Tx.Commit] or [Tx.Rollback], all operations on the transaction fail with [ErrTxDone].

The statements prepared for a transaction by calling the transaction's [Tx.Prepare] or [Tx.Stmt] methods are closed by the call to [Tx.Commit] or [Tx.Rollback].

**Hierarchy**

- Tx

## Methods

### commit

```ts
commit(): void
```

Commit commits the transaction.

**Returns void**

### exec

```ts
exec(query, ...args): sql.Result
```

Exec executes a query that doesn't return rows. For example: an INSERT and UPDATE.

Exec uses [context.Background] internally; to specify the context, use [Tx.ExecContext].

**Parameters**

- `query: string`
- `Rest ...args: any[]`

**Returns sql.Result**

### execContext

```ts
execContext(ctx, query, ...args): sql.Result
```

ExecContext executes a query that doesn't return rows. For example: an INSERT and UPDATE.

**Parameters**

- `ctx: context.Context`
- `query: string`
- `Rest ...args: any[]`

**Returns sql.Result**

### prepare

```ts
prepare(query): Stmt
```

Prepare creates a prepared statement for use within a transaction.

The returned statement operates within the transaction and will be closed when the transaction has been committed or rolled back.

To use an existing prepared statement on this transaction, see [Tx.Stmt].

Prepare uses [context.Background] internally; to specify the context, use [Tx.PrepareContext].

**Parameters**

- `query: string`

**Returns Stmt**

### prepareContext

```ts
prepareContext(ctx, query): Stmt
```

PrepareContext creates a prepared statement for use within a transaction.

The returned statement operates within the transaction and will be closed when the transaction has been committed or rolled back.

To use an existing prepared statement on this transaction, see [Tx.Stmt].

The provided context will be used for the preparation of the context, not for the execution of the returned statement. The returned statement will run in the transaction context.

**Parameters**

- `ctx: context.Context`
- `query: string`

**Returns Stmt**

### query

```ts
query(query, ...args): sql.Rows
```

Query executes a query that returns rows, typically a SELECT.

Query uses [context.Background] internally; to specify the context, use [Tx.QueryContext].

**Parameters**

- `query: string`
- `Rest ...args: any[]`

**Returns sql.Rows**

### queryContext

```ts
queryContext(ctx, query, ...args): sql.Rows
```

QueryContext executes a query that returns rows, typically a SELECT.

**Parameters**

- `ctx: context.Context`
- `query: string`
- `Rest ...args: any[]`

**Returns sql.Rows**

### queryRow

```ts
queryRow(query, ...args): Row
```

QueryRow executes a query that is expected to return at most one row. QueryRow always returns a non-nil value. Errors are deferred until [Row]'s Scan method is called. If the query selects no rows, the [*Row.Scan] will return [ErrNoRows]. Otherwise, the [*Row.Scan] scans the first selected row and discards the rest.

QueryRow uses [context.Background] internally; to specify the context, use [Tx.QueryRowContext].

**Parameters**

- `query: string`
- `Rest ...args: any[]`

**Returns Row**

### queryRowContext

```ts
queryRowContext(ctx, query, ...args): Row
```

QueryRowContext executes a query that is expected to return at most one row. QueryRowContext always returns a non-nil value. Errors are deferred until [Row]'s Scan method is called. If the query selects no rows, the [*Row.Scan] will return [ErrNoRows]. Otherwise, the [*Row.Scan] scans the first selected row and discards the rest.

**Parameters**

- `ctx: context.Context`
- `query: string`
- `Rest ...args: any[]`

**Returns Row**

### rollback

```ts
rollback(): void
```

Rollback aborts the transaction.

**Returns void**

### stmt

```ts
stmt(stmt): Stmt
```

Stmt returns a transaction-specific prepared statement from an existing statement.

Example:

```
 updateMoney, err := db.Prepare("UPDATE balance SET money=money+? WHERE id=?")
 ...
 tx, err := db.Begin()
 ...
 res, err := tx.Stmt(updateMoney).Exec(123.45, 98293203)
```

The returned statement operates within the transaction and will be closed when the transaction has been committed or rolled back.

Stmt uses [context.Background] internally; to specify the context, use [Tx.StmtContext].

**Parameters**

- `stmt: Stmt`

**Returns Stmt**

### stmtContext

```ts
stmtContext(ctx, stmt): Stmt
```

StmtContext returns a transaction-specific prepared statement from an existing statement.

Example:

```
 updateMoney, err := db.Prepare("UPDATE balance SET money=money+? WHERE id=?")
 ...
 tx, err := db.Begin()
 ...
 res, err := tx.StmtContext(ctx, updateMoney).Exec(123.45, 98293203)
```

The provided context is used for the preparation of the statement, not for the execution of the statement.

The returned statement operates within the transaction and will be closed when the transaction has been committed or rolled back.

**Parameters**

- `ctx: context.Context`
- `stmt: Stmt`

**Returns Stmt**
