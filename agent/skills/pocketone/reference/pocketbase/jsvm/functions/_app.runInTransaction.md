Source: https://pocketbase.io/jsvm/functions/_app.runInTransaction.html

# Function runInTransaction

```ts
runInTransaction(fn): void
```

RunInTransaction wraps fn into a transaction for the regular app database.

It is safe to nest RunInTransaction calls as long as you use the callback's txApp.

**Parameters**

- `fn: ((txApp) => void)`

**Returns void**
