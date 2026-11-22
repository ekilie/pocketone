Source: https://pocketbase.io/jsvm/interfaces/router.Router.html

# Interface Router<T>

Router defines a thin wrapper around the standard Go [http.ServeMux] by adding support for routing sub-groups, middlewares and other common utils.

Example:

```
  r := NewRouter[*MyEvent](eventFactory)

  // middlewares
  r.BindFunc(m1, m2)

  // routes
  r.GET("/test", handler1)

  // sub-routers/groups
  api := r.Group("/api")
  api.GET("/admins", handler2)

  // generate a http.ServeMux instance based on the router configurations
  mux, _ := r.BuildMux()

  http.ListenAndServe("localhost:8090", mux)
```

**Type Parameters**

- T

**Hierarchy**

- [_sAebeNE](https://pocketbase.io/jsvm/types/router._sAebeNE.html)<T>

  - Router

## Methods

### any

```ts
any(path, action): Route<T>
```

Any is a shorthand for [RouterGroup.AddRoute] with "" as route method (aka. matches any method).

**Parameters**

- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### bind

```ts
bind(...middlewares): RouterGroup<T>
```

Bind registers one or multiple middleware handlers to the current group.

**Parameters**

- `Rest ...middlewares: hook.Handler<T>[]`

**Returns RouterGroup<T>**

### bindFunc

```ts
bindFunc(...middlewareFuncs): RouterGroup<T>
```

BindFunc registers one or multiple middleware functions to the current group.

The registered middleware functions are "anonymous" and with default priority, aka. executes in the order they were registered.

If you need to specify a named middleware (ex. so that it can be removed) or middleware with custom exec prirority, use [RouterGroup.Bind] method.

**Parameters**

- `Rest ...middlewareFuncs: ((e) => void)[]`

**Returns RouterGroup<T>**

### buildMux

```ts
buildMux(): http.Handler
```

BuildMux constructs a new mux [http.Handler] instance from the current router configurations.

**Returns http.Handler**

### delete

```ts
delete(path, action): Route<T>
```

DELETE is a shorthand for [RouterGroup.AddRoute] with DELETE as route method.

**Parameters**

- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### get

```ts
get(path, action): Route<T>
```

GET is a shorthand for [RouterGroup.AddRoute] with GET as route method.

**Parameters**

- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### group

```ts
group(prefix): RouterGroup<T>
```

Group creates and register a new child Group into the current one with the specified prefix.

The prefix follows the standard Go net/http ServeMux pattern format ("[HOST]/[PATH]") and will be concatenated recursively into the final route path, meaning that only the root level group could have HOST as part of the prefix.

Returns the newly created group to allow chaining and registering sub-routes and group specific middlewares.

**Parameters**

- `prefix: string`

**Returns RouterGroup<T>**

### hasRoute

```ts
hasRoute(method, path): boolean
```

HasRoute checks whether the specified route pattern (method + path) is registered in the current group or its children.

This could be useful to conditionally register and checks for routes in order prevent panic on duplicated routes.

Note that routes with anonymous and named wildcard placeholder are treated as equal, aka. "GET /abc/" is considered the same as "GET /abc/{something...}".

**Parameters**

- `method: string`
- `path: string`

**Returns boolean**

### head

```ts
head(path, action): Route<T>
```

HEAD is a shorthand for [RouterGroup.AddRoute] with HEAD as route method.

**Parameters**

- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### options

```ts
options(path, action): Route<T>
```

OPTIONS is a shorthand for [RouterGroup.AddRoute] with OPTIONS as route method.

**Parameters**

- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### patch

```ts
patch(path, action): Route<T>
```

PATCH is a shorthand for [RouterGroup.AddRoute] with PATCH as route method.

**Parameters**

- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### post

```ts
post(path, action): Route<T>
```

POST is a shorthand for [RouterGroup.AddRoute] with POST as route method.

**Parameters**

- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### put

```ts
put(path, action): Route<T>
```

PUT is a shorthand for [RouterGroup.AddRoute] with PUT as route method.

**Parameters**

- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### route

```ts
route(method, path, action): Route<T>
```

Route registers a single route into the current group.

Note that the final route path will be the concatenation of all parent groups prefixes + the route path. The path follows the standard Go net/http ServeMux format ("[HOST]/[PATH]"), meaning that only a top level group route could have HOST as part of the prefix.

Returns the newly created route to allow attaching route-only middlewares.

**Parameters**

- `method: string`
- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### search

```ts
search(path, action): Route<T>
```

SEARCH is a shorthand for [RouterGroup.AddRoute] with SEARCH as route method.

**Parameters**

- `path: string`
- `action: ((e) => void)`

**Returns Route<T>**

### unbind

```ts
unbind(...middlewareIds): RouterGroup<T>
```

Unbind removes one or more middlewares with the specified id(s) from the current group and its children (if any).

Anonymous middlewares are not removable, aka. this method does nothing if the middleware id is an empty string.

**Parameters**

- `Rest ...middlewareIds: string[]`

**Returns RouterGroup<T>**

## Properties

### middlewares

```ts
middlewares: hook.Handler<T>[]
```

### prefix

```ts
prefix: string
```
