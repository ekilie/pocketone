Source: https://pocketbase.io/jsvm/functions/_apis.enrichRecord.html

# Function enrichRecord

```ts
enrichRecord(e, record, ...defaultExpands): void
```

EnrichRecord parses the request context and enrich the provided record:

```
  - expands relations (if defaultExpands and/or ?expand query param is set)
  - ensures that the emails of the auth record and its expanded auth relations
    are visible only for the current logged superuser, record owner or record with manage access
```

**Parameters**

- `e: RequestEvent`
- `record: core.Record`
- `Rest ...defaultExpands: string[]`

**Returns void**
