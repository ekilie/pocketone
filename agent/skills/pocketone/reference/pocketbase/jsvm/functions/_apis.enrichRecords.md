Source: https://pocketbase.io/jsvm/functions/_apis.enrichRecords.html

# Function enrichRecords

```ts
enrichRecords(e, records, ...defaultExpands): void
```

EnrichRecords parses the request context and enriches the provided records:

```
  - expands relations (if defaultExpands and/or ?expand query param is set)
  - ensures that the emails of the auth records and their expanded auth relations
    are visible only for the current logged superuser, record owner or record with manage access
```

Note: Expects all records to be from the same collection!

**Parameters**

- `e: RequestEvent`
- `records: core.Record[]`
- `Rest ...defaultExpands: string[]`

**Returns void**
