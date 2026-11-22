Source: https://pocketbase.io/jsvm/interfaces/dbx.ModelQuery.html

# Interface ModelQuery

ModelQuery represents a query associated with a struct model.

**Hierarchy**

- ModelQuery

## Methods

### context

```ts
context(): context.Context
```

Context returns the context associated with the query.

**Returns context.Context**

### delete

```ts
delete(): void
```

Delete deletes a row in the table using the primary key specified by the struct model associated with this query.

**Returns void**

### exclude

```ts
exclude(...attrs): ModelQuery
```

Exclude excludes the specified struct fields from being inserted/updated into the DB table.

**Parameters**

- `Rest ...attrs: string[]`

**Returns ModelQuery**

### insert

```ts
insert(...attrs): void
```

Insert inserts a row in the table using the struct model associated with this query.

By default, it inserts *all* public fields into the table, including those nil or empty ones. You may pass a list of the fields to this method to indicate that only those fields should be inserted. You may also call Exclude to exclude some fields from being inserted.

If a model has an empty primary key, it is considered auto-incremental and the corresponding struct field will be filled with the generated primary key value after a successful insertion.

**Parameters**

- `Rest ...attrs: string[]`

**Returns void**

### update

```ts
update(...attrs): void
```

Update updates a row in the table using the struct model associated with this query. The row being updated has the same primary key as specified by the model.

By default, it updates *all* public fields in the table, including those nil or empty ones. You may pass a list of the fields to this method to indicate that only those fields should be updated. You may also call Exclude to exclude some fields from being updated.

**Parameters**

- `Rest ...attrs: string[]`

**Returns void**

### withContext

```ts
withContext(ctx): ModelQuery
```

WithContext associates a context with the query.

**Parameters**

- `ctx: context.Context`

**Returns ModelQuery**
