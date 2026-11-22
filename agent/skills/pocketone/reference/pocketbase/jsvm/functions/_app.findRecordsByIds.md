Source: https://pocketbase.io/jsvm/functions/_app.findRecordsByIds.html

# Function findRecordsByIds

```ts
findRecordsByIds(collectionModelOrIdentifier, recordIds, ...optFilters): core.Record[]
```

FindRecordsByIds finds all records by the specified ids. If no records are found, returns an empty slice.

**Parameters**

- `collectionModelOrIdentifier: any`
- `recordIds: string[]`
- `Rest ...optFilters: ((q) => void)[]`

**Returns core.Record[]**
