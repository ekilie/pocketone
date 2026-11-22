Source: https://pocketbase.io/jsvm/functions/routerUse.html

# Function routerUse

```ts
routerUse(...middlewares): void
```

RouterUse registers one or more global middlewares that are executed along the handler middlewares after a matching route is found.

Example:

```js
routerUse((e) => {
  console.log(e.request.url.path)
  return e.next()
})
```

*Note that this method is available only in pb_hooks context.*

**Parameters**

- `Rest ...middlewares: (string | Middleware | ((e) => void))[]`

**Returns void**
