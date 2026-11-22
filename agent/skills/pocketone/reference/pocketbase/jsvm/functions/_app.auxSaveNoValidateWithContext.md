Source: https://pocketbase.io/jsvm/functions/_app.auxSaveNoValidateWithContext.html

# Function auxSaveNoValidateWithContext

```ts
auxSaveNoValidateWithContext(ctx, model): void
```

AuxSaveNoValidateWithContext is the same as [App.AuxSaveNoValidate()] but allows specifying a context to limit the db execution.

If you want to also run validations before persisting, use [App.AuxSaveWithContext()].

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**
