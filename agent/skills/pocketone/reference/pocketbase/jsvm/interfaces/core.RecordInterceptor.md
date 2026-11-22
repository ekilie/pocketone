Source: https://pocketbase.io/jsvm/interfaces/core.RecordInterceptor.html

# Interface RecordInterceptor

RecordInterceptor defines a field interface for reacting to various Record related operations (create, delete, validate, etc.).

**Hierarchy**

- RecordInterceptor

**Indexable**

```ts
[key: string]: any
```

## Methods

### intercept

```ts
intercept(ctx, app, record, actionName, actionFunc): void
```

Interceptor is invoked when a specific record action occurs allowing you to perform extra validations and normalization (ex. uploading or deleting files).

Note that users must call actionFunc() manually if they want to execute the specific record action.

**Parameters**

- `ctx: context.Context`
- `app: App`
- `record: core.Record`
- `actionName: string`
- `actionFunc: (() => void)`

**Returns void**
