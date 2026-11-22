Source: https://pocketbase.io/jsvm/functions/_app.deleteTable.html

# Function deleteTable

```ts
deleteTable(dangerousTableName): void
```

DeleteTable drops the specified table.

This method is a no-op if a table with the provided name doesn't exist.

NB! Be aware that this method is vulnerable to SQL injection and the "dangerousTableName" argument must come only from trusted input!

**Parameters**

- `dangerousTableName: string`

**Returns void**
