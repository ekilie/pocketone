Source: https://pocketbase.io/jsvm/functions/_app.unsafeWithoutHooks.html

# Function unsafeWithoutHooks

```ts
unsafeWithoutHooks(): App
```

UnsafeWithoutHooks returns a shallow copy of the current app WITHOUT any registered hooks.

NB! Note that using the returned app instance may cause data integrity errors since the Record validations and data normalizations (including files uploads) rely on the app hooks to work.

**Returns App**
