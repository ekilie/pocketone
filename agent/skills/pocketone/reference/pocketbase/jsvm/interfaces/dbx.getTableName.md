Source: https://pocketbase.io/jsvm/interfaces/dbx.getTableName.html

# Interface getTableName

**Hierarchy**

- getTableName

```ts
getTableName(a): string
```

GetTableName implements the default way of determining the table name corresponding to the given model struct or slice of structs. To get the actual table name for a model, you should use DB.TableMapFunc() instead. Do not call this method in a model's TableName() method because it will cause infinite loop.

**Parameters**

- `a: {}`

**Returns string**
