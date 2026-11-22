Source: https://pocketbase.io/jsvm/functions/_app.importCollections.html

# Function importCollections

```ts
importCollections(toImport, deleteMissing): void
```

ImportCollections imports the provided collections data in a single transaction.

For existing matching collections, the imported data is unmarshaled on top of the existing model.

NB! If deleteMissing is true, ALL NON-SYSTEM COLLECTIONS AND SCHEMA FIELDS, that are not present in the imported configuration, WILL BE DELETED (this includes their related records data).

**Parameters**

- `toImport: _TygojaDict[]`
- `deleteMissing: boolean`

**Returns void**
