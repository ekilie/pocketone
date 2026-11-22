Source: https://pocketbase.io/jsvm/functions/_app.nonconcurrentDB.html

# Function nonconcurrentDB

```ts
nonconcurrentDB(): Builder
```

NonconcurrentDB returns the nonconcurrent app data.db builder instance.

The returned db instance is limited only to a single open connection, meaning that it can process only 1 db operation at a time (other queries queue up).

This method is used mainly internally and in the tests to execute write (save/delete) db operations as it helps with minimizing the SQLITE_BUSY errors.

Most users should use simply DB() as it will automatically route the query execution to ConcurrentDB() or NonconcurrentDB().

In a transaction the ConcurrentDB() and NonconcurrentDB() refer to the same *dbx.TX instance.

**Returns Builder**
