Source: https://pocketbase.io/jsvm/interfaces/forms.newRecordUpsert.html

# Interface newRecordUpsert

**Hierarchy**

- newRecordUpsert

```ts
newRecordUpsert(app, record): RecordUpsert
```

NewRecordUpsert creates a new [RecordUpsert] form from the provided [CoreApp] and [core.Record] instances (for create you could pass a pointer to an empty Record - core.NewRecord(collection)).

**Parameters**

- `app: excludeHooks<App>`
- `record: core.Record`

**Returns RecordUpsert**
