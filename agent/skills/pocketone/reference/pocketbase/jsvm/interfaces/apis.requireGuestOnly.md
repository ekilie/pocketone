Source: https://pocketbase.io/jsvm/interfaces/apis.requireGuestOnly.html

# Interface requireGuestOnly

**Hierarchy**

- requireGuestOnly

```ts
requireGuestOnly(): hook.Handler<RequestEvent>
```

RequireGuestOnly middleware requires a request to NOT have a valid Authorization header.

This middleware is the opposite of [apis.RequireAuth()].

**Returns hook.Handler<RequestEvent>**
