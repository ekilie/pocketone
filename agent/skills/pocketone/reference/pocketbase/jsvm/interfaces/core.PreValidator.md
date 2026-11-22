Source: https://pocketbase.io/jsvm/interfaces/core.PreValidator.html

# Interface PreValidator

PreValidator defines an optional model interface for registering a function that will run BEFORE firing the validation hooks (see [App.ValidateWithContext]).

**Hierarchy**

- PreValidator

**Indexable**

```ts
[key: string]: any
```

## Methods

### preValidate

```ts
preValidate(ctx, app): void
```

PreValidate defines a function that runs BEFORE the validation hooks.

**Parameters**

- `ctx: context.Context`
- `app: App`

**Returns void**
