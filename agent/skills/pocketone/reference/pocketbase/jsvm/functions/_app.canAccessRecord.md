Source: https://pocketbase.io/jsvm/functions/_app.canAccessRecord.html

# Function canAccessRecord

```ts
canAccessRecord(record, requestInfo, accessRule): boolean
```

CanAccessRecord checks if a record is allowed to be accessed by the specified requestInfo and accessRule.

Rule and db checks are ignored in case requestInfo.Auth is a superuser.

The returned error indicate that something unexpected happened during the check (eg. invalid rule or db query error).

The method always return false on invalid rule or db query error.

Example:

```
 requestInfo, _ := e.RequestInfo()
 record, _ := app.FindRecordById("example", "RECORD_ID")
 rule := types.Pointer("@request.auth.id != '' || status = 'public'")
 // ... or use one of the record collection's rule, eg. record.Collection().ViewRule

 if ok, _ := app.CanAccessRecord(record, requestInfo, rule); ok { ... }
```

**Parameters**

- `record: core.Record`
- `requestInfo: RequestInfo`
- `accessRule: string`

**Returns boolean**
