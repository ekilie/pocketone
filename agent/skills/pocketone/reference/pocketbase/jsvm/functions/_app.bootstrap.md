Source: https://pocketbase.io/jsvm/functions/_app.bootstrap.html

# Function bootstrap

```ts
bootstrap(): void
```

Bootstrap initializes the application (aka. create data dir, open db connections, load settings, etc.).

It calls ClearBootstrap() if the application was already bootstrapped.

**Returns void**
