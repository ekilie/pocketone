Source: https://pocketbase.io/jsvm/functions/_app.createViewFields.html

# Function createViewFields

```ts
createViewFields(dangerousSelectQuery): core.FieldsList
```

CreateViewFields creates a new FieldsList from the provided select query.

There are some caveats:

- The select query must have an "id" column.
- Wildcard ("*") columns are not supported to avoid accidentally leaking sensitive data.

NB! Be aware that this method is vulnerable to SQL injection and the "dangerousSelectQuery" argument must come only from trusted input!

**Parameters**

- `dangerousSelectQuery: string`

**Returns core.FieldsList**
