Source: https://pocketbase.io/jsvm/functions/_app.saveNoValidateWithContext.html

# Function saveNoValidateWithContext

```ts
saveNoValidateWithContext(ctx, model): void
```

SaveNoValidateWithContext is the same as [App.SaveNoValidate()] but allows specifying a context to limit the db execution.

If you want to also run validations before persisting, use [App.SaveWithContext()].

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**
