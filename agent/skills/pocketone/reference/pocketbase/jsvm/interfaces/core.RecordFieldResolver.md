Source: https://pocketbase.io/jsvm/interfaces/core.RecordFieldResolver.html

# Interface RecordFieldResolver

RecordFieldResolver defines a custom search resolver struct for managing Record model search fields.

Usually used together with `search.Provider`. Example:

```
  resolver := resolvers.NewRecordFieldResolver(
      app,
      myCollection,
      &models.RequestInfo{...},
      true,
  )
  provider := search.NewProvider(resolver)
  ...
```

**Hierarchy**

- RecordFieldResolver

## Methods

### allowHiddenFields

```ts
allowHiddenFields(): boolean
```

AllowHiddenFields returns whether the current resolver allows filtering hidden fields.

**Returns boolean**

### allowedFields

```ts
allowedFields(): string[]
```

AllowedFields returns a copy of the resolver's allowed fields.

**Returns string[]**

### resolve

```ts
resolve(fieldName): ResolverResult
```

Resolve implements `search.FieldResolver` interface.

Example of some resolvable fieldName formats:

```
 id
 someSelect.each
 project.screen.status
 screen.project_via_prototype.name
 @request.context
 @request.method
 @request.query.filter
 @request.headers.x_token
 @request.auth.someRelation.name
 @request.body.someRelation.name
 @request.body.someField
 @request.body.someSelect:each
 @request.body.someField:isset
 @collection.product.name
```

**Parameters**

- `fieldName: string`

**Returns ResolverResult**

### setAllowHiddenFields

```ts
setAllowHiddenFields(allowHiddenFields): void
```

SetAllowHiddenFields enables or disables hidden fields filtering.

**Parameters**

- `allowHiddenFields: boolean`

**Returns void**

### setAllowedFields

```ts
setAllowedFields(newAllowedFields): void
```

SetAllowedFields replaces the resolver's allowed fields with the new ones.

**Parameters**

- `newAllowedFields: string[]`

**Returns void**

### updateQuery

```ts
updateQuery(query): void
```

**Parameters**

- `query: SelectQuery`

**Returns void**

### Todo

think of a better a way how to call it automatically after BuildExpr

UpdateQuery implements `search.FieldResolver` interface.

Conditionally updates the provided search query based on the resolved fields (eg. dynamically joining relations).
