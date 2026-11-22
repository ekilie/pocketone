Source: https://pocketbase.io/jsvm/functions/_app.logger.html

# Function logger

```ts
logger(): Logger
```

Logger returns the default app logger.

If the application is not bootstrapped yet, fallbacks to slog.Default().

**Returns Logger**
