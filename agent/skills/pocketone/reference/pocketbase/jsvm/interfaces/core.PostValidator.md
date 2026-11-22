Source: https://pocketbase.io/jsvm/interfaces/core.PostValidator.html

# Interface PostValidator

PostValidator defines an optional model interface for registering a function that will run AFTER executing the validation hooks (see [App.ValidateWithContext]).

**Hierarchy**

- PostValidator

**Indexable**

```ts
[key: string]: any
```

## Methods

### postValidate

```ts
postValidate(ctx, app): void
```

PostValidate defines a function that runs AFTER the successful execution of the validation hooks.

**Parameters**

- `ctx: context.Context`
- `app: App`

**Returns void**
