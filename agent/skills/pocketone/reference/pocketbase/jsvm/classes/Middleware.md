Source: https://pocketbase.io/jsvm/classes/Middleware.html

# Class Middleware

Middleware defines a single request middleware handler.

This class is usually used when you want to explicitly specify a priority to your custom route middleware.

Example:

```js
routerUse(new Middleware((e) => {
  console.log(e.request.url.path)
  return e.next()
}, -10))
```

**Hierarchy**

- Middleware

## Constructors

### constructor

```ts
new Middleware(func, priority?, id?): Middleware
```

**Parameters**

- `func: string | ((e) => void)`
- `Optional priority: number`
- `Optional id: string`

**Returns Middleware**
