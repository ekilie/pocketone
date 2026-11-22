Source: https://pocketbase.io/jsvm/functions/_apis.requireAuth.html

# Function requireAuth

```ts
requireAuth(...optCollectionNames): hook.Handler<RequestEvent>
```

RequireAuth middleware requires a request to have a valid record Authorization header.

The auth record could be from any collection. You can further filter the allowed record auth collections by specifying their names.

Example:

```
 apis.RequireAuth()                      // any auth collection
 apis.RequireAuth("_superusers", "users") // only the listed auth collections
```

**Parameters**

- `Rest ...optCollectionNames: string[]`

**Returns hook.Handler<RequestEvent>**
