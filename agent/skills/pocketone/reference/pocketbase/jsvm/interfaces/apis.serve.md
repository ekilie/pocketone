Source: https://pocketbase.io/jsvm/interfaces/apis.serve.html

# Interface serve

**Hierarchy**

- serve

```ts
serve(app, config): void
```

Serve starts a new app web server.

NB! The app should be bootstrapped before starting the web server.

Example:

```
 app.Bootstrap()
 apis.Serve(app, apis.ServeConfig{
   HttpAddr:        "127.0.0.1:8080",
   ShowStartBanner: false,
 })
```

**Parameters**

- `app: excludeHooks<App>`
- `config: ServeConfig`

**Returns void**
