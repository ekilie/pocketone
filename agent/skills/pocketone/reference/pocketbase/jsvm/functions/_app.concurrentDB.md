Source: https://pocketbase.io/jsvm/functions/_app.concurrentDB.html

# Function concurrentDB

```ts
concurrentDB(): Builder
```

ConcurrentDB returns the concurrent app data.db builder instance.

This method is used mainly internally for executing db read operations in a concurrent/non-blocking manner.

Most users should use simply DB() as it will automatically route the query execution to ConcurrentDB() or NonconcurrentDB().

In a transaction the ConcurrentDB() and NonconcurrentDB() refer to the same *dbx.TX instance.

**Returns Builder**
