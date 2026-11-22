Source: https://pocketbase.io/jsvm/interfaces/dbx.OciBuilder.html

# Interface OciBuilder

OciBuilder is the builder for Oracle databases.

**Hierarchy**

- [_sLZfOMn](https://pocketbase.io/jsvm/types/dbx._sLZfOMn.html)

  - OciBuilder

## Methods

### addColumn

```ts
addColumn(table, col, typ): Query
```

AddColumn creates a Query that can be used to add a column to a table.

**Parameters**

- `table: string`
- `col: string`
- `typ: string`

**Returns Query**

### addForeignKey

```ts
addForeignKey(table, name, cols, refCols, refTable, ...options): Query
```

AddForeignKey creates a Query that can be used to add a foreign key constraint to a table. The length of cols and refCols must be the same as they refer to the primary and referential columns. The optional "options" parameters will be appended to the SQL statement. They can be used to specify options such as "ON DELETE CASCADE".

**Parameters**

- `table: string`
- `name: string`
- `cols: string[]`
- `refCols: string[]`
- `refTable: string`
- `Rest ...options: string[]`

**Returns Query**

### addPrimaryKey

```ts
addPrimaryKey(table, name, ...cols): Query
```

AddPrimaryKey creates a Query that can be used to specify primary key(s) for a table. The "name" parameter specifies the name of the primary key constraint.

**Parameters**

- `table: string`
- `name: string`
- `Rest ...cols: string[]`

**Returns Query**

### alterColumn

```ts
alterColumn(table, col, typ): Query
```

AlterColumn creates a Query that can be used to change the definition of a table column.

**Parameters**

- `table: string`
- `col: string`
- `typ: string`

**Returns Query**

### createIndex

```ts
createIndex(table, name, ...cols): Query
```

CreateIndex creates a Query that can be used to create an index for a table.

**Parameters**

- `table: string`
- `name: string`
- `Rest ...cols: string[]`

**Returns Query**

### createTable

```ts
createTable(table, cols, ...options): Query
```

CreateTable creates a Query that represents a CREATE TABLE SQL statement. The keys of cols are the column names, while the values of cols are the corresponding column types. The optional "options" parameters will be appended to the generated SQL statement.

**Parameters**

- `table: string`
- `cols: _TygojaDict`
- `Rest ...options: string[]`

**Returns Query**

### createUniqueIndex

```ts
createUniqueIndex(table, name, ...cols): Query
```

CreateUniqueIndex creates a Query that can be used to create a unique index for a table.

**Parameters**

- `table: string`
- `name: string`
- `Rest ...cols: string[]`

**Returns Query**

### db

```ts
db(): dbx.DB
```

DB returns the DB instance that this builder is associated with.

**Returns dbx.DB**

### delete

```ts
delete(table, where): Query
```

Delete creates a Query that represents a DELETE SQL statement. If the "where" expression is nil, the DELETE SQL statement will have no WHERE clause (be careful in this case as the SQL statement will delete ALL rows in the table).

**Parameters**

- `table: string`
- `where: Expression`

**Returns Query**

### dropColumn

```ts
dropColumn(table, col): Query
```

DropColumn creates a Query that can be used to drop a column from a table.

**Parameters**

- `table: string`
- `col: string`

**Returns Query**

### dropForeignKey

```ts
dropForeignKey(table, name): Query
```

DropForeignKey creates a Query that can be used to remove the named foreign key constraint from a table.

**Parameters**

- `table: string`
- `name: string`

**Returns Query**

### dropIndex

```ts
dropIndex(table, name): Query
```

DropIndex creates a Query that can be used to remove the named index from a table.

**Parameters**

- `table: string`
- `name: string`

**Returns Query**

### dropPrimaryKey

```ts
dropPrimaryKey(table, name): Query
```

DropPrimaryKey creates a Query that can be used to remove the named primary key constraint from a table.

**Parameters**

- `table: string`
- `name: string`

**Returns Query**

### dropTable

```ts
dropTable(table): Query
```

DropTable creates a Query that can be used to drop a table.

**Parameters**

- `table: string`

**Returns Query**

### executor

```ts
executor(): Executor
```

Executor returns the executor object (a DB instance or a transaction) for executing SQL statements.

**Returns Executor**

### generatePlaceholder

```ts
generatePlaceholder(i): string
```

GeneratePlaceholder generates an anonymous parameter placeholder with the given parameter ID.

**Parameters**

- `i: number`

**Returns string**

### insert

```ts
insert(table, cols): Query
```

Insert creates a Query that represents an INSERT SQL statement. The keys of cols are the column names, while the values of cols are the corresponding column values to be inserted.

**Parameters**

- `table: string`
- `cols: Params`

**Returns Query**

### model

```ts
model(model): ModelQuery
```

Model returns a new ModelQuery object that can be used to perform model-based DB operations. The model passed to this method should be a pointer to a model struct.

**Parameters**

- `model: {}`

**Returns ModelQuery**

### newQuery

```ts
newQuery(sql): Query
```

NewQuery creates a new Query object with the given SQL statement. The SQL statement may contain parameter placeholders which can be bound with actual parameter values before the statement is executed.

**Parameters**

- `sql: string`

**Returns Query**

### queryBuilder

```ts
queryBuilder(): QueryBuilder
```

QueryBuilder returns the query builder supporting the current DB.

**Returns QueryBuilder**

### quote

```ts
quote(s): string
```

Quote quotes a string so that it can be embedded in a SQL statement as a string value.

**Parameters**

- `s: string`

**Returns string**

### quoteSimpleColumnName

```ts
quoteSimpleColumnName(s): string
```

QuoteSimpleColumnName quotes a simple column name. A simple column name does not contain any table prefix.

**Parameters**

- `s: string`

**Returns string**

### quoteSimpleTableName

```ts
quoteSimpleTableName(s): string
```

QuoteSimpleTableName quotes a simple table name. A simple table name does not contain any schema prefix.

**Parameters**

- `s: string`

**Returns string**

### renameColumn

```ts
renameColumn(table, oldName, newName): Query
```

RenameColumn creates a Query that can be used to rename a column in a table.

**Parameters**

- `table: string`
- `oldName: string`
- `newName: string`

**Returns Query**

### renameTable

```ts
renameTable(oldName, newName): Query
```

RenameTable creates a Query that can be used to rename a table.

**Parameters**

- `oldName: string`
- `newName: string`

**Returns Query**

### select

```ts
select(...cols): SelectQuery
```

Select returns a new SelectQuery object that can be used to build a SELECT statement. The parameters to this method should be the list column names to be selected. A column name may have an optional alias name. For example, Select("id", "my_name AS name").

**Parameters**

- `Rest ...cols: string[]`

**Returns SelectQuery**

### truncateTable

```ts
truncateTable(table): Query
```

TruncateTable creates a Query that can be used to truncate a table.

**Parameters**

- `table: string`

**Returns Query**

### update

```ts
update(table, cols, where): Query
```

Update creates a Query that represents an UPDATE SQL statement. The keys of cols are the column names, while the values of cols are the corresponding new column values. If the "where" expression is nil, the UPDATE SQL statement will have no WHERE clause (be careful in this case as the SQL statement will update ALL rows in the table).

**Parameters**

- `table: string`
- `cols: Params`
- `where: Expression`

**Returns Query**

### upsert

```ts
upsert(table, cols, ...constraints): Query
```

Upsert creates a Query that represents an UPSERT SQL statement. Upsert inserts a row into the table if the primary key or unique index is not found. Otherwise it will update the row with the new values. The keys of cols are the column names, while the values of cols are the corresponding column values to be inserted.

**Parameters**

- `table: string`
- `cols: Params`
- `Rest ...constraints: string[]`

**Returns Query**
