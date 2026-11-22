Source: https://pocketbase.io/jsvm/functions/_app.saveWithContext.html

# Function saveWithContext

```ts
saveWithContext(ctx, model): void
```

SaveWithContext is the same as [App.Save()] but allows specifying a context to limit the db execution.

If you don't want to run validations, use [App.SaveNoValidateWithContext()].

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**
