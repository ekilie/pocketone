Source: https://pocketbase.io/jsvm/functions/_app.auxConcurrentDB.html

# Function auxConcurrentDB

```ts
auxConcurrentDB(): Builder
```

AuxConcurrentDB returns the concurrent app auxiliary.db builder instance.

This method is used mainly internally for executing db read operations in a concurrent/non-blocking manner.

Most users should use simply AuxDB() as it will automatically route the query execution to AuxConcurrentDB() or AuxNonconcurrentDB().

In a transaction the AuxConcurrentDB() and AuxNonconcurrentDB() refer to the same *dbx.TX instance.

**Returns Builder**
