Source: https://pocketbase.io/jsvm/interfaces/forms.RecordUpsert.html

# Interface RecordUpsert

**Hierarchy**

- RecordUpsert

  - [RecordUpsertForm](https://pocketbase.io/jsvm/classes/RecordUpsertForm.html)

**Implemented by**

- [RecordUpsertForm](https://pocketbase.io/jsvm/classes/RecordUpsertForm.html)

## Methods

### drySubmit

```ts
drySubmit(callback): void
```

Deprecated: It was previously used as part of the record create action but it is not needed anymore and will be removed in the future.

DrySubmit performs a temp form submit within a transaction and reverts it at the end. For actual record persistence, check the [RecordUpsert.Submit()] method.

This method doesn't perform validations, handle file uploads/deletes or trigger app save events!

**Parameters**

- `callback: ((txApp, drySavedRecord) => void)`

**Returns void**

### grantManagerAccess

```ts
grantManagerAccess(): void
```

GrantManagerAccess updates the form access level to "manager" allowing directly changing some system record fields (often used with auth collection records).

**Returns void**

### grantSuperuserAccess

```ts
grantSuperuserAccess(): void
```

GrantSuperuserAccess updates the form access level to "superuser" allowing directly changing all system record fields, including those marked as "Hidden".

**Returns void**

### hasManageAccess

```ts
hasManageAccess(): boolean
```

HasManageAccess reports whether the form has "manager" or "superuser" level access.

**Returns boolean**

### load

```ts
load(data): void
```

Load loads the provided data into the form and the related record.

**Parameters**

- `data: _TygojaDict`

**Returns void**

### resetAccess

```ts
resetAccess(): void
```

ResetAccess resets the form access level to the accessLevelDefault.

**Returns void**

### setApp

```ts
setApp(app): void
```

SetApp replaces the current form app instance.

This could be used for example if you want to change at later stage before submission to change from regular -> transactional app instance.

**Parameters**

- `app: excludeHooks<App>`

**Returns void**

### setContext

```ts
setContext(ctx): void
```

SetContext assigns ctx as context of the current form.

**Parameters**

- `ctx: context.Context`

**Returns void**

### setRecord

```ts
setRecord(record): void
```

SetRecord replaces the current form record instance.

**Parameters**

- `record: core.Record`

**Returns void**

### submit

```ts
submit(): void
```

Submit validates the form specific validations and attempts to save the form record.

**Returns void**
