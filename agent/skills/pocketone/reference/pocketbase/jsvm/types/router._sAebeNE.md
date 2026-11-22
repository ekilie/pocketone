Source: https://pocketbase.io/jsvm/types/router._sAebeNE.html

# Type alias _sAebeNE<T>

```ts
_sAebeNE<T>: RouterGroup<T>
```

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
