Source: https://pocketbase.io/jsvm/functions/_app.findFirstRecordByFilter.html

# Function findFirstRecordByFilter

```ts
findFirstRecordByFilter(collectionModelOrIdentifier, filter, ...params): core.Record
```

FindFirstRecordByFilter returns the first available record matching the provided filter (if any).

NB! Use the last params argument to bind untrusted user variables!

Returns sql.ErrNoRows if no record is found.

Example:

```
 app.FindFirstRecordByFilter("posts", "")
 app.FindFirstRecordByFilter("posts", "slug={:slug} && status='public'", dbx.Params{"slug": "test"})
```

**Parameters**

- `collectionModelOrIdentifier: any`
- `filter: string`
- `Rest ...params: Params[]`

**Returns core.Record**
