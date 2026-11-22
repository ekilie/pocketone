Source: https://pocketbase.io/jsvm/functions/_app.auxSaveWithContext.html

# Function auxSaveWithContext

```ts
auxSaveWithContext(ctx, model): void
```

AuxSaveWithContext is the same as [App.AuxSave()] but allows specifying a context to limit the db execution.

If you don't want to run validations, use [App.AuxSaveNoValidateWithContext()].

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**
