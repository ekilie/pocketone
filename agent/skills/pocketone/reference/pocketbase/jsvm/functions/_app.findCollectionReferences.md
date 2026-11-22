Source: https://pocketbase.io/jsvm/functions/_app.findCollectionReferences.html

# Function findCollectionReferences

```ts
findCollectionReferences(collection, ...excludeIds): _TygojaDict
```

FindCollectionReferences returns information for all relation fields referencing the provided collection.

If the provided collection has reference to itself then it will be also included in the result. To exclude it, pass the collection id as the excludeIds argument.

**Parameters**

- `collection: core.Collection`
- `Rest ...excludeIds: string[]`

**Returns _TygojaDict**
