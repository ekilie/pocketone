Source: https://pocketbase.io/jsvm/functions/_app.expandRecords.html

# Function expandRecords

```ts
expandRecords(records, expands, optFetchFunc): _TygojaDict
```

ExpandRecords expands the relations of the provided Record models list.

If optFetchFunc is not set, then a default function will be used that returns all relation records.

Returns a map with the failed expand parameters and their errors.

**Parameters**

- `records: core.Record[]`
- `expands: string[]`
- `optFetchFunc: ExpandFetchFunc`

**Returns _TygojaDict**
