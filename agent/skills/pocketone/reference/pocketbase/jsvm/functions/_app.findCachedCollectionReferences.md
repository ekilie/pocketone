Source: https://pocketbase.io/jsvm/functions/_app.findCachedCollectionReferences.html

# Function findCachedCollectionReferences

```ts
findCachedCollectionReferences(collection, ...excludeIds): _TygojaDict
```

FindCachedCollectionReferences is similar to [App.FindCollectionReferences] but retrieves the Collection from the app cache instead of making a db call.

NB! This method is suitable for read-only Collection operations.

If you plan making changes to the returned Collection model, use [App.FindCollectionReferences] instead.

Caveats:

```
  - The returned Collection should be used only for read-only operations.
    Avoid directly modifying the returned cached Collection as it will affect
    the global cached value even if you don't persist the changes in the database!
  - If you are updating a Collection in a transaction and then call this method before commit,
    it'll return the cached Collection state and not the one from the uncommitted transaction.
  - The cache is automatically updated on collections db change (create/update/delete).
    To manually reload the cache you can call [App.ReloadCachedCollections].
```

**Parameters**

- `collection: core.Collection`
- `Rest ...excludeIds: string[]`

**Returns _TygojaDict**
