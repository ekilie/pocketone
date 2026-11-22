Source: https://pocketbase.io/jsvm/functions/_app.isCollectionNameUnique.html

# Function isCollectionNameUnique

```ts
isCollectionNameUnique(name, ...excludeIds): boolean
```

IsCollectionNameUnique checks that there is no existing collection with the provided name (case insensitive!).

Note: case insensitive check because the name is used also as table name for the records.

**Parameters**

- `name: string`
- `Rest ...excludeIds: string[]`

**Returns boolean**
