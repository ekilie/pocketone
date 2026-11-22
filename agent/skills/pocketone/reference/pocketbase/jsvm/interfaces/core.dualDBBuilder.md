Source: https://pocketbase.io/jsvm/interfaces/core.dualDBBuilder.html

# Interface dualDBBuilder

note: expects both builder to use the same driver

**Hierarchy**

- dualDBBuilder

## Methods

### addColumn

```ts
addColumn(table, col, typ): Query
```

AddColumn implements the [dbx.Builder.AddColumn] interface method.

**Parameters**

- `table: string`
- `col: string`
- `typ: string`

**Returns Query**

### addForeignKey

```ts
addForeignKey(table, name, cols, refCols, refTable, ...options): Query
```

AddForeignKey implements the [dbx.Builder.AddForeignKey] interface method.

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

AddPrimaryKey implements the [dbx.Builder.AddPrimaryKey] interface method.

**Parameters**

- `table: string`
- `name: string`
- `Rest ...cols: string[]`

**Returns Query**

### alterColumn

```ts
alterColumn(table, col, typ): Query
```

AlterColumn implements the [dbx.Builder.AlterColumn] interface method.

**Parameters**

- `table: string`
- `col: string`
- `typ: string`

**Returns Query**

### createIndex

```ts
createIndex(table, name, ...cols): Query
```

CreateIndex implements the [dbx.Builder.CreateIndex] interface method.

**Parameters**

- `table: string`
- `name: string`
- `Rest ...cols: string[]`

**Returns Query**

### createTable

```ts
createTable(table, cols, ...options): Query
```

CreateTable implements the [dbx.Builder.CreateTable] interface method.

**Parameters**

- `table: string`
- `cols: _TygojaDict`
- `Rest ...options: string[]`

**Returns Query**

### createUniqueIndex

```ts
createUniqueIndex(table, name, ...cols): Query
```

CreateUniqueIndex implements the [dbx.Builder.CreateUniqueIndex] interface method.

**Parameters**

- `table: string`
- `name: string`
- `Rest ...cols: string[]`

**Returns Query**

### delete

```ts
delete(table, where): Query
```

Delete implements the [dbx.Builder.Delete] interface method.

**Parameters**

- `table: string`
- `where: Expression`

**Returns Query**

### dropColumn

```ts
dropColumn(table, col): Query
```

DropColumn implements the [dbx.Builder.DropColumn] interface method.

**Parameters**

- `table: string`
- `col: string`

**Returns Query**

### dropForeignKey

```ts
dropForeignKey(table, name): Query
```

DropForeignKey implements the [dbx.Builder.DropForeignKey] interface method.

**Parameters**

- `table: string`
- `name: string`

**Returns Query**

### dropIndex

```ts
dropIndex(table, name): Query
```

DropIndex implements the [dbx.Builder.DropIndex] interface method.

**Parameters**

- `table: string`
- `name: string`

**Returns Query**

### dropPrimaryKey

```ts
dropPrimaryKey(table, name): Query
```

DropPrimaryKey implements the [dbx.Builder.DropPrimaryKey] interface method.

**Parameters**

- `table: string`
- `name: string`

**Returns Query**

### dropTable

```ts
dropTable(table): Query
```

DropTable implements the [dbx.Builder.DropTable] interface method.

**Parameters**

- `table: string`

**Returns Query**

### generatePlaceholder

```ts
generatePlaceholder(i): string
```

GeneratePlaceholder implements the [dbx.Builder.GeneratePlaceholder] interface method.

**Parameters**

- `i: number`

**Returns string**

### insert

```ts
insert(table, cols): Query
```

Insert implements the [dbx.Builder.Insert] interface method.

**Parameters**

- `table: string`
- `cols: Params`

**Returns Query**

### model

```ts
model(data): ModelQuery
```

Model implements the [dbx.Builder.Model] interface method.

**Parameters**

- `data: {}`

**Returns ModelQuery**

### newQuery

```ts
newQuery(str): Query
```

NewQuery implements the [dbx.Builder.NewQuery] interface method by routing the SELECT queries to the concurrent builder instance.

**Parameters**

- `str: string`

**Returns Query**

### queryBuilder

```ts
queryBuilder(): QueryBuilder
```

QueryBuilder implements the [dbx.Builder.QueryBuilder] interface method.

**Returns QueryBuilder**

### quote

```ts
quote(str): string
```

Quote implements the [dbx.Builder.Quote] interface method.

**Parameters**

- `str: string`

**Returns string**

### quoteSimpleColumnName

```ts
quoteSimpleColumnName(col): string
```

QuoteSimpleColumnName implements the [dbx.Builder.QuoteSimpleColumnName] interface method.

**Parameters**

- `col: string`

**Returns string**

### quoteSimpleTableName

```ts
quoteSimpleTableName(table): string
```

QuoteSimpleTableName implements the [dbx.Builder.QuoteSimpleTableName] interface method.

**Parameters**

- `table: string`

**Returns string**

### renameColumn

```ts
renameColumn(table, oldName, newName): Query
```

RenameColumn implements the [dbx.Builder.RenameColumn] interface method.

**Parameters**

- `table: string`
- `oldName: string`
- `newName: string`

**Returns Query**

### renameTable

```ts
renameTable(oldName, newName): Query
```

RenameTable implements the [dbx.Builder.RenameTable] interface method.

**Parameters**

- `oldName: string`
- `newName: string`

**Returns Query**

### select

```ts
select(...cols): SelectQuery
```

Select implements the [dbx.Builder.Select] interface method.

**Parameters**

- `Rest ...cols: string[]`

**Returns SelectQuery**

### truncateTable

```ts
truncateTable(table): Query
```

TruncateTable implements the [dbx.Builder.TruncateTable] interface method.

**Parameters**

- `table: string`

**Returns Query**

### update

```ts
update(table, cols, where): Query
```

Update implements the [dbx.Builder.Update] interface method.

**Parameters**

- `table: string`
- `cols: Params`
- `where: Expression`

**Returns Query**

### upsert

```ts
upsert(table, cols, ...constraints): Query
```

Upsert implements the [dbx.Builder.Upsert] interface method.

**Parameters**

- `table: string`
- `cols: Params`
- `Rest ...constraints: string[]`

**Returns Query**
