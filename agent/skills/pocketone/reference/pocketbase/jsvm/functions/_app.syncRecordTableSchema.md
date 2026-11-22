Source: https://pocketbase.io/jsvm/functions/_app.syncRecordTableSchema.html

# Function syncRecordTableSchema

```ts
syncRecordTableSchema(newCollection, oldCollection): void
```

SyncRecordTableSchema compares the two provided collections and applies the necessary related record table changes.

If oldCollection is null, then only newCollection is used to create the record table.

This method is automatically invoked as part of a collection create/update/delete operation.

**Parameters**

- `newCollection: core.Collection`
- `oldCollection: core.Collection`

**Returns void**
