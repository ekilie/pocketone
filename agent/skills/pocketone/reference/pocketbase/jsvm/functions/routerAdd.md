Source: https://pocketbase.io/jsvm/functions/routerAdd.html

# Function routerAdd

```ts
routerAdd(method, path, handler, ...middlewares): void
```

RouterAdd registers a new route definition.

Example:

```js
routerAdd("GET", "/hello", (e) => {
    return e.json(200, {"message": "Hello!"})
}, $apis.requireAuth())
```

*Note that this method is available only in pb_hooks context.*

**Parameters**

- `method: string`
- `path: string`
- `handler: ((e) => void)`
- `Rest ...middlewares: (string | ((e) => void) | Middleware)[]`

**Returns void**
