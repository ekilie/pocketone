Source: https://pocketbase.io/jsvm/functions/_app.recordQuery.html

# Function recordQuery

```ts
recordQuery(collectionModelOrIdentifier): SelectQuery
```

RecordQuery returns a new Record select query from a collection model, id or name.

In case a collection id or name is provided and that collection doesn't actually exists, the generated query will be created with a cancelled context and will fail once an executor (Row(), One(), All(), etc.) is called.

**Parameters**

- `collectionModelOrIdentifier: any`

**Returns SelectQuery**
