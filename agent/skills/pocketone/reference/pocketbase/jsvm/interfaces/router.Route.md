Source: https://pocketbase.io/jsvm/interfaces/router.Route.html

# Interface Route<T>

**Type Parameters**

- T

**Hierarchy**

- Route

## Methods

### bind

```ts
bind(...middlewares): Route<T>
```

Bind registers one or multiple middleware handlers to the current route.

**Parameters**

- `Rest ...middlewares: hook.Handler<T>[]`

**Returns Route<T>**

### bindFunc

```ts
bindFunc(...middlewareFuncs): Route<T>
```

BindFunc registers one or multiple middleware functions to the current route.

The registered middleware functions are "anonymous" and with default priority, aka. executes in the order they were registered.

If you need to specify a named middleware (ex. so that it can be removed) or middleware with custom exec prirority, use the [Route.Bind] method.

**Parameters**

- `Rest ...middlewareFuncs: ((e) => void)[]`

**Returns Route<T>**

### unbind

```ts
unbind(...middlewareIds): Route<T>
```

Unbind removes one or more middlewares with the specified id(s) from the current route.

It also adds the removed middleware ids to an exclude list so that they could be skipped from the execution chain in case the middleware is registered in a parent group.

Anonymous middlewares are considered non-removable, aka. this method does nothing if the middleware id is an empty string.

**Parameters**

- `Rest ...middlewareIds: string[]`

**Returns Route<T>**

## Properties

### action

```ts
action: ((e) => void)
```

**Type declaration**

```ts
(e): void
```

**Parameters**

- `e: T`

**Returns void**

### method

```ts
method: string
```

### middlewares

```ts
middlewares: hook.Handler<T>[]
```

### path

```ts
path: string
```
