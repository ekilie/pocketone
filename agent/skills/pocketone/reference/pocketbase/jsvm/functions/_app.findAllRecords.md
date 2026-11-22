Source: https://pocketbase.io/jsvm/functions/_app.findAllRecords.html

# Function findAllRecords

```ts
findAllRecords(collectionModelOrIdentifier, ...exprs): core.Record[]
```

FindAllRecords finds all records matching specified db expressions.

Returns all collection records if no expression is provided.

Returns an empty slice if no records are found.

Example:

```
 // no extra expressions
 app.FindAllRecords("example")

 // with extra expressions
 expr1 := dbx.HashExp{"email": "test@example.com"}
 expr2 := dbx.NewExp("LOWER(username) = {:username}", dbx.Params{"username": "test"})
 app.FindAllRecords("example", expr1, expr2)
```

**Parameters**

- `collectionModelOrIdentifier: any`
- `Rest ...exprs: Expression[]`

**Returns core.Record[]**
