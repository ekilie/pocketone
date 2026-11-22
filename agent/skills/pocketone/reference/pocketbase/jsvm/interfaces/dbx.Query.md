Source: https://pocketbase.io/jsvm/interfaces/dbx.Query.html

# Interface Query

Query represents a SQL statement to be executed.

**Hierarchy**

- Query

## Methods

### all

```ts
all(slice): void
```

All executes the SQL statement and populates all the resulting rows into a slice of struct or NullStringMap. The slice must be given as a pointer. Each slice element must be either a struct or a NullStringMap. Refer to Rows.ScanStruct() and Rows.ScanMap() for more details on how each slice element can be. If the query returns no row, the slice will be an empty slice (not nil).

**Parameters**

- `slice: {}`

**Returns void**

### bind

```ts
bind(params): Query
```

Bind sets the parameters that should be bound to the SQL statement. The parameter placeholders in the SQL statement are in the format of "{:ParamName}".

**Parameters**

- `params: Params`

**Returns Query**

### close

```ts
close(): void
```

Close closes the underlying prepared statement. Close does nothing if the query has not been prepared before.

**Returns void**

### column

```ts
column(a): void
```

Column executes the SQL statement and populates the first column of the result into a slice. Note that the parameter must be a pointer to a slice.

**Parameters**

- `a: {}`

**Returns void**

### context

```ts
context(): context.Context
```

Context returns the context associated with the query.

**Returns context.Context**

### execute

```ts
execute(): sql.Result
```

Execute executes the SQL statement without retrieving data.

**Returns sql.Result**

### one

```ts
one(a): void
```

One executes the SQL statement and populates the first row of the result into a struct or NullStringMap. Refer to Rows.ScanStruct() and Rows.ScanMap() for more details on how to specify the variable to be populated. Note that when the query has no rows in the result set, an sql.ErrNoRows will be returned.

**Parameters**

- `a: {}`

**Returns void**

### params

```ts
params(): Params
```

Params returns the parameters to be bound to the SQL statement represented by this query.

**Returns Params**

### prepare

```ts
prepare(): Query
```

Prepare creates a prepared statement for later queries or executions. Close() should be called after finishing all queries.

**Returns Query**

### row

```ts
row(...a): void
```

Row executes the SQL statement and populates the first row of the result into a list of variables. Note that the number of the variables should match to that of the columns in the query result. Note that when the query has no rows in the result set, an sql.ErrNoRows will be returned.

**Parameters**

- `Rest ...a: {}[]`

**Returns void**

### rows

```ts
rows(): dbx.Rows
```

Rows executes the SQL statement and returns a Rows object to allow retrieving data row by row.

**Returns dbx.Rows**

### sql

```ts
sql(): string
```

SQL returns the original SQL used to create the query. The actual SQL (RawSQL) being executed is obtained by replacing the named parameter placeholders with anonymous ones.

**Returns string**

### withAllHook

```ts
withAllHook(fn): Query
```

WithOneHook associates the provided hook function with the query, called on q.All(), allowing you to implement custom slice scan based on the All() argument and/or result.

**Parameters**

- `fn: AllHookFunc`

**Returns Query**

### withContext

```ts
withContext(ctx): Query
```

WithContext associates a context with the query.

**Parameters**

- `ctx: context.Context`

**Returns Query**

### withExecHook

```ts
withExecHook(fn): Query
```

WithExecHook associates the provided exec hook function with the query.

It is called for every Query resolver (Execute(), One(), All(), Row(), Column()), allowing you to implement auto fail/retry or any other additional handling.

**Parameters**

- `fn: ExecHookFunc`

**Returns Query**

### withOneHook

```ts
withOneHook(fn): Query
```

WithOneHook associates the provided hook function with the query, called on q.One(), allowing you to implement custom struct scan based on the One() argument and/or result.

**Parameters**

- `fn: OneHookFunc`

**Returns Query**

## Properties

### execLogFunc

```ts
execLogFunc: ExecLogFunc
```

ExecLogFunc is called each time when a SQL statement is executed.

### fieldMapper

```ts
fieldMapper: FieldMapFunc
```

FieldMapper maps struct field names to DB column names.

### lastError

```ts
lastError: Error
```

LastError contains the last error (if any) of the query. LastError is cleared by Execute(), Row(), Rows(), One(), and All().

### logFunc

```ts
logFunc: LogFunc
```

LogFunc is used to log the SQL statement being executed.

### perfFunc

```ts
perfFunc: PerfFunc
```

PerfFunc is used to log the SQL execution time. It is ignored if nil. Deprecated: Please use QueryLogFunc and ExecLogFunc instead.

### queryLogFunc

```ts
queryLogFunc: QueryLogFunc
```

QueryLogFunc is called each time when performing a SQL query that returns data.
