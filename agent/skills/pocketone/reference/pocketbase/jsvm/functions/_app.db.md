Source: https://pocketbase.io/jsvm/functions/_app.db.html

# Function db

```ts
db(): Builder
```

DB returns the default app data.db builder instance.

To minimize SQLITE_BUSY errors, it automatically routes the SELECT queries to the underlying concurrent db pool and everything else to the nonconcurrent one.

For more finer control over the used connections pools you can call directly ConcurrentDB() or NonconcurrentDB().

**Returns Builder**
