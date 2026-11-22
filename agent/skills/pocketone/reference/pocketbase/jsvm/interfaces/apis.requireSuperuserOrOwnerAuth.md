Source: https://pocketbase.io/jsvm/interfaces/apis.requireSuperuserOrOwnerAuth.html

# Interface requireSuperuserOrOwnerAuth

**Hierarchy**

- requireSuperuserOrOwnerAuth

```ts
requireSuperuserOrOwnerAuth(ownerIdPathParam): hook.Handler<RequestEvent>
```

RequireSuperuserOrOwnerAuth middleware requires a request to have a valid superuser or regular record owner Authorization header set.

This middleware is similar to [apis.RequireAuth()] but for the auth record token expects to have the same id as the path parameter ownerIdPathParam (default to "id" if empty).

**Parameters**

- `ownerIdPathParam: string`

**Returns hook.Handler<RequestEvent>**
