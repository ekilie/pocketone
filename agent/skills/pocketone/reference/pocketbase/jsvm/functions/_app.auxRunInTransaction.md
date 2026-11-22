Source: https://pocketbase.io/jsvm/functions/_app.auxRunInTransaction.html

# Function auxRunInTransaction

```ts
auxRunInTransaction(fn): void
```

AuxRunInTransaction wraps fn into a transaction for the auxiliary app database.

It is safe to nest RunInTransaction calls as long as you use the callback's txApp.

**Parameters**

- `fn: ((txApp) => void)`

**Returns void**
