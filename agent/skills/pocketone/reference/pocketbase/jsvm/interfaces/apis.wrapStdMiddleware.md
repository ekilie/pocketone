Source: https://pocketbase.io/jsvm/interfaces/apis.wrapStdMiddleware.html

# Interface wrapStdMiddleware

**Hierarchy**

- wrapStdMiddleware

```ts
wrapStdMiddleware(m): ((_arg0) => void)
```

WrapStdMiddleware wraps Go [func(http.Handler) http.Handle] into a PocketBase middleware func.

**Parameters**

- `m: ((_arg0) => http.Handler)`

**Returns ((_arg0) => void)**

```ts
(_arg0): void
```

WrapStdMiddleware wraps Go [func(http.Handler) http.Handle] into a PocketBase middleware func.

**Parameters**

- `_arg0: RequestEvent`

**Returns void**
