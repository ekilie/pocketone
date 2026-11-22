Source: https://pocketbase.io/jsvm/functions/_app.auxDB.html

# Function auxDB

```ts
auxDB(): Builder
```

AuxDB returns the app auxiliary.db builder instance.

To minimize SQLITE_BUSY errors, it automatically routes the SELECT queries to the underlying concurrent db pool and everything else to the nonconcurrent one.

For more finer control over the used connections pools you can call directly AuxConcurrentDB() or AuxNonconcurrentDB().

**Returns Builder**
