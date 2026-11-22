Source: https://pocketbase.io/jsvm/functions/_app.findRecordsByFilter.html

# Function findRecordsByFilter

```ts
findRecordsByFilter(collectionModelOrIdentifier, filter, sort, limit, offset, ...params): core.Record[]
```

FindRecordsByFilter returns limit number of records matching the provided string filter.

NB! Use the last "params" argument to bind untrusted user variables!

The filter argument is optional and can be empty string to target all available records.

The sort argument is optional and can be empty string OR the same format used in the web APIs, ex. "-created,title".

If the limit argument is <= 0, no limit is applied to the query and all matching records are returned.

Returns an empty slice if no records are found.

Example:

```
 app.FindRecordsByFilter(
   "posts",
   "title ~ {:title} && visible = {:visible}",
   "-created",
   10,
   0,
   dbx.Params{"title": "lorem ipsum", "visible": true}
 )
```

**Parameters**

- `collectionModelOrIdentifier: any`
- `filter: string`
- `sort: string`
- `limit: number`
- `offset: number`
- `Rest ...params: Params[]`

**Returns core.Record[]**
