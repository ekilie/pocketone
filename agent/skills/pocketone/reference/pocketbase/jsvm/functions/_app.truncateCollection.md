Source: https://pocketbase.io/jsvm/functions/_app.truncateCollection.html

# Function truncateCollection

```ts
truncateCollection(collection): void
```

TruncateCollection deletes all records associated with the provided collection.

The truncate operation is executed in a single transaction, aka. either everything is deleted or none.

Note that this method will also trigger the records related cascade and file delete actions.

**Parameters**

- `collection: core.Collection`

**Returns void**
