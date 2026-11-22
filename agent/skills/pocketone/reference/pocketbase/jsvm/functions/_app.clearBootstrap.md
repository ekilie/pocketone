Source: https://pocketbase.io/jsvm/functions/_app.clearBootstrap.html

# Function clearBootstrap

```ts
clearBootstrap(): void
```

ClearBootstrap releases the initialized core app resources (closing db connections, stopping cron ticker, etc.).

This method is no-op if the application is not bootstrapped yet.

**Returns void**
