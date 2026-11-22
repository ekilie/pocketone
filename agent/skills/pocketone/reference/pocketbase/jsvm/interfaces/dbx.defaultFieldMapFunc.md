Source: https://pocketbase.io/jsvm/interfaces/dbx.defaultFieldMapFunc.html

# Interface defaultFieldMapFunc

**Hierarchy**

- defaultFieldMapFunc

```ts
defaultFieldMapFunc(f): string
```

DefaultFieldMapFunc maps a field name to a DB column name. The mapping rule set by this method is that words in a field name will be separated by underscores and the name will be turned into lower case. For example, "FirstName" maps to "first_name", and "MyID" becomes "my_id". See DB.FieldMapper for more details.

**Parameters**

- `f: string`

**Returns string**
