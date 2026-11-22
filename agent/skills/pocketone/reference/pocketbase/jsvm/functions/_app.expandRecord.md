Source: https://pocketbase.io/jsvm/functions/_app.expandRecord.html

# Function expandRecord

```ts
expandRecord(record, expands, optFetchFunc): _TygojaDict
```

ExpandRecord expands the relations of a single Record model.

If optFetchFunc is not set, then a default function will be used that returns all relation records.

Returns a map with the failed expand parameters and their errors.

**Parameters**

- `record: core.Record`
- `expands: string[]`
- `optFetchFunc: ExpandFetchFunc`

**Returns _TygojaDict**
