Source: https://pocketbase.io/jsvm/interfaces/core.BaseRecordProxy.html

# Interface BaseRecordProxy

BaseRecordProxy implements the [RecordProxy] interface and it is intended to be used as embed to custom user provided Record proxy structs.

**Hierarchy**

- [_svuoMfa](https://pocketbase.io/jsvm/types/core._svuoMfa.html)

  - BaseRecordProxy

## Methods

### baseFilesPath

```ts
baseFilesPath(): string
```

BaseFilesPath returns the storage dir path used by the record.

**Returns string**

### clone

```ts
clone(): core.Record
```

Clone returns a shallow copy of the current record model with all of its collection and unknown fields data, expand and flags copied.

use [Record.Fresh()] instead if you want a copy with only the latest collection fields data and everything else reset to the defaults.

**Returns core.Record**

### collection

```ts
collection(): core.Collection
```

Collection returns the Collection model associated with the current Record model.

NB! The returned collection is only for read purposes and it shouldn't be modified because it could have unintended side-effects on other Record models from the same collection.

**Returns core.Collection**

### customData

```ts
customData(): _TygojaDict
```

CustomData returns a shallow copy ONLY of the custom record fields data, aka. fields that are neither defined by the collection, nor special system ones.

Note that custom fields prefixed with "@pbInternal" are always skipped.

**Returns _TygojaDict**

### dbExport

```ts
dbExport(app): _TygojaDict
```

DBExport implements the [DBExporter] interface and returns a key-value map with the data to be persisted when saving the Record in the database.

**Parameters**

- `app: App`

**Returns _TygojaDict**

### email

```ts
email(): string
```

Email returns the "email" record field value (usually available with Auth collections).

**Returns string**

### emailVisibility

```ts
emailVisibility(): boolean
```

Verified returns the "emailVisibility" record field value (usually available with Auth collections).

**Returns boolean**

### expand

```ts
expand(): _TygojaDict
```

Expand returns a shallow copy of the current Record model expand data (if any).

**Returns _TygojaDict**

### expandedAll

```ts
expandedAll(relField): core.Record[]
```

ExpandedAll retrieves a slice of relation Records from the already loaded expand data of the current model.

If the requested expand relation is single, this method normalizes the return result and will wrap the single model as a slice.

Returns nil slice if there is no such expand relation loaded.

**Parameters**

- `relField: string`

**Returns core.Record[]**

### expandedOne

```ts
expandedOne(relField): core.Record
```

ExpandedOne retrieves a single relation Record from the already loaded expand data of the current model.

If the requested expand relation is multiple, this method returns only first available Record from the expanded relation.

Returns nil if there is no such expand relation loaded.

**Parameters**

- `relField: string`

**Returns core.Record**

### fieldsData

```ts
fieldsData(): _TygojaDict
```

FieldsData returns a shallow copy ONLY of the collection's fields record's data.

**Returns _TygojaDict**

### findFileFieldByFile

```ts
findFileFieldByFile(filename): core.FileField
```

FindFileFieldByFile returns the first file type field for which any of the record's data contains the provided filename.

**Parameters**

- `filename: string`

**Returns core.FileField**

### fresh

```ts
fresh(): core.Record
```

Fresh returns a shallow copy of the current record model populated with its LATEST data state and everything else reset to the defaults (aka. no expand, no unknown fields and with default visibility flags).

**Returns core.Record**

### get

```ts
get(key): any
```

Get returns a normalized single record model data value for "key".

**Parameters**

- `key: string`

**Returns any**

### getBool

```ts
getBool(key): boolean
```

GetBool returns the data value for "key" as a bool.

**Parameters**

- `key: string`

**Returns boolean**

### getDateTime

```ts
getDateTime(key): types.DateTime
```

GetDateTime returns the data value for "key" as a DateTime instance.

**Parameters**

- `key: string`

**Returns types.DateTime**

### getFloat

```ts
getFloat(key): number
```

GetFloat returns the data value for "key" as a float64.

**Parameters**

- `key: string`

**Returns number**

### getGeoPoint

```ts
getGeoPoint(key): GeoPoint
```

GetGeoPoint returns the data value for "key" as a GeoPoint instance.

**Parameters**

- `key: string`

**Returns GeoPoint**

### getInt

```ts
getInt(key): number
```

GetInt returns the data value for "key" as an int.

**Parameters**

- `key: string`

**Returns number**

### getInt64

```ts
getInt64(key): number
```

GetInt64 returns the data value for "key" as an int64.

**Parameters**

- `key: string`

**Returns number**

### getRaw

```ts
getRaw(key): any
```

**Parameters**

- `key: string`

**Returns any**

### getString

```ts
getString(key): string
```

GetString returns the data value for "key" as a string.

**Parameters**

- `key: string`

**Returns string**

### getStringSlice

```ts
getStringSlice(key): string[]
```

GetStringSlice returns the data value for "key" as a slice of non-zero unique strings.

**Parameters**

- `key: string`

**Returns string[]**

### getUnsavedFiles

```ts
getUnsavedFiles(key): filesystem.File[]
```

GetUnsavedFiles returns the uploaded files for the provided "file" field key, (aka. the current [*filesystem.File] values) so that you can apply further validations or modifications (including changing the file name or content before persisting).

Example:

```
 files := record.GetUnsavedFiles("documents")
 for _, f := range files {
     f.Name = "doc_" + f.Name // add a prefix to each file name
 }
 app.Save(record) // the files are pointers so the applied changes will transparently reflect on the record value
```

**Parameters**

- `key: string`

**Returns filesystem.File[]**

### getUploadedFiles

```ts
getUploadedFiles(key): filesystem.File[]
```

Deprecated: replaced with GetUnsavedFiles.

**Parameters**

- `key: string`

**Returns filesystem.File[]**

### hide

```ts
hide(...fieldNames): core.Record
```

Hide hides the specified fields from the public safe serialization of the record.

**Parameters**

- `Rest ...fieldNames: string[]`

**Returns core.Record**

### hookTags

```ts
hookTags(): string[]
```

HookTags returns the hook tags associated with the current record.

**Returns string[]**

### ignoreEmailVisibility

```ts
ignoreEmailVisibility(state): core.Record
```

IgnoreEmailVisibility toggles the flag to ignore the auth record email visibility check.

**Parameters**

- `state: boolean`

**Returns core.Record**

### ignoreUnchangedFields

```ts
ignoreUnchangedFields(state): core.Record
```

IgnoreUnchangedFields toggles the flag to ignore the unchanged fields from the DB export for the UPDATE SQL query.

This could be used if you want to save only the record fields that you've changed without overwrite other untouched fields in case of concurrent update.

Note that the fields change comparison is based on the current fields against m.Original() (aka. if you have performed save on the same Record instance multiple times you may have to refetch it, so that m.Original() could reflect the last saved change).

**Parameters**

- `state: boolean`

**Returns core.Record**

### isNew

```ts
isNew(): boolean
```

IsNew indicates what type of db query (insert or update) should be used with the model instance.

**Returns boolean**

### isSuperuser

```ts
isSuperuser(): boolean
```

IsSuperuser returns whether the current record is a superuser, aka. whether the record is from the _superusers collection.

**Returns boolean**

### lastSavedPK

```ts
lastSavedPK(): any
```

LastSavedPK returns the last saved primary key of the model.

Its value is updated to the latest PK value after MarkAsNotNew() or PostScan() calls.

**Returns any**

### load

```ts
load(data): void
```

Load bulk loads the provided data into the current Record model.

**Parameters**

- `data: _TygojaDict`

**Returns void**

### markAsNew

```ts
markAsNew(): void
```

MarkAsNew clears the pk field and marks the current model as "new" (aka. forces m.IsNew() to be true).

**Returns void**

### markAsNotNew

```ts
markAsNotNew(): void
```

MarkAsNew set the pk field to the Id value and marks the current model as NOT "new" (aka. forces m.IsNew() to be false).

**Returns void**

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements the [json.Marshaler] interface.

Only the data exported by `PublicExport()` will be serialized.

**Returns string | number[]**

### mergeExpand

```ts
mergeExpand(expand): void
```

MergeExpand merges recursively the provided expand data into the current model's expand (if any).

Note that if an expanded prop with the same key is a slice (old or new expand) then both old and new records will be merged into a new slice (aka. a :merge: [b,c] => [a,b,c]). Otherwise the "old" expanded record will be replace with the "new" one (aka. a :merge: aNew => aNew).

**Parameters**

- `expand: _TygojaDict`

**Returns void**

### newAuthToken

```ts
newAuthToken(): string
```

NewAuthToken generates and returns a new record authentication token.

**Returns string**

### newEmailChangeToken

```ts
newEmailChangeToken(newEmail): string
```

NewEmailChangeToken generates and returns a new auth record change email request token.

**Parameters**

- `newEmail: string`

**Returns string**

### newFileToken

```ts
newFileToken(): string
```

NewFileToken generates and returns a new record private file access token.

**Returns string**

### newPasswordResetToken

```ts
newPasswordResetToken(): string
```

NewPasswordResetToken generates and returns a new auth record password reset request token.

**Returns string**

### newStaticAuthToken

```ts
newStaticAuthToken(duration): string
```

NewStaticAuthToken generates and returns a new static record authentication token.

Static auth tokens are similar to the regular auth tokens, but are non-refreshable and support custom duration.

Zero or negative duration will fallback to the duration from the auth collection settings.

**Parameters**

- `duration: Duration`

**Returns string**

### newVerificationToken

```ts
newVerificationToken(): string
```

NewVerificationToken generates and returns a new record verification token.

**Returns string**

### original

```ts
original(): core.Record
```

Original returns a shallow copy of the current record model populated with its ORIGINAL db data state (aka. right after PostScan()) and everything else reset to the defaults.

If record was created using NewRecord() the original will be always a blank record (until PostScan() is invoked).

**Returns core.Record**

### pk

```ts
pk(): any
```

**Returns any**

### postScan

```ts
postScan(): void
```

PostScan implements the [dbx.PostScanner] interface.

It essentially refreshes/updates the current Record original state as if the model was fetched from the databases for the first time.

Or in other words, it means that m.Original().FieldsData() will have the same values as m.Record().FieldsData().

**Returns void**

### proxyRecord

```ts
proxyRecord(): core.Record
```

ProxyRecord returns the proxied Record model.

**Returns core.Record**

### publicExport

```ts
publicExport(): _TygojaDict
```

PublicExport exports only the record fields that are safe to be public.

To export unknown data fields you need to set record.WithCustomData(true).

For auth records, to force the export of the email field you need to set record.IgnoreEmailVisibility(true).

**Returns _TygojaDict**

### refreshTokenKey

```ts
refreshTokenKey(): void
```

RefreshTokenKey generates and sets a new random auth record "tokenKey".

**Returns void**

### replaceModifiers

```ts
replaceModifiers(data): _TygojaDict
```

ReplaceModifiers returns a new map with applied modifier values based on the current record and the specified data.

The resolved modifier keys will be removed.

Multiple modifiers will be applied one after another, while reusing the previous base key value result (ex. 1; -5; +2 => -2).

Note that because Go doesn't guaranteed the iteration order of maps, we would explicitly apply shorter keys first for a more consistent and reproducible behavior.

Example usage:

```
  newData := record.ReplaceModifiers(data)
   // record: {"field": 10}
   // data:   {"field+": 5}
   // result: {"field": 15}
```

**Parameters**

- `data: _TygojaDict`

**Returns _TygojaDict**

### set

```ts
set(key, value): void
```

Set sets the provided key-value data pair into the current Record model.

If the record collection has field with name matching the provided "key", the value will be further normalized according to the field setter(s).

**Parameters**

- `key: string`
- `value: any`

**Returns void**

### setEmail

```ts
setEmail(email): void
```

SetEmail sets the "email" record field value (usually available with Auth collections).

**Parameters**

- `email: string`

**Returns void**

### setEmailVisibility

```ts
setEmailVisibility(visible): void
```

SetEmailVisibility sets the "emailVisibility" record field value (usually available with Auth collections).

**Parameters**

- `visible: boolean`

**Returns void**

### setExpand

```ts
setExpand(expand): void
```

SetExpand replaces the current Record's expand with the provided expand arg data (shallow copied).

**Parameters**

- `expand: _TygojaDict`

**Returns void**

### setIfFieldExists

```ts
setIfFieldExists(key, value): core.Field
```

SetIfFieldExists sets the provided key-value data pair into the current Record model ONLY if key is existing Collection field name/modifier.

This method does nothing if key is not a known Collection field name/modifier.

On success returns the matched Field, otherwise - nil.

To set any key-value, including custom/unknown fields, use the [Record.Set] method.

**Parameters**

- `key: string`
- `value: any`

**Returns core.Field**

### setPassword

```ts
setPassword(password): void
```

SetPassword sets the "password" record field value (usually available with Auth collections).

**Parameters**

- `password: string`

**Returns void**

### setProxyRecord

```ts
setProxyRecord(record): void
```

SetProxyRecord loads the specified record model into the current proxy.

**Parameters**

- `record: core.Record`

**Returns void**

### setRandomPassword

```ts
setRandomPassword(): string
```

SetRandomPassword sets the "password" auth record field to a random autogenerated value.

The autogenerated password is ~30 characters and it is set directly as hash, aka. the field plain password value validators (length, pattern, etc.) are ignored (this is usually used as part of the auto created OTP or OAuth2 user flows).

**Returns string**

### setRaw

```ts
setRaw(key, value): void
```

Set sets the provided key-value data pair into the current Record model directly as it is WITHOUT NORMALIZATIONS.

See also [Record.Set].

**Parameters**

- `key: string`
- `value: any`

**Returns void**

### setTokenKey

```ts
setTokenKey(key): void
```

SetTokenKey sets the "tokenKey" record field value (usually available with Auth collections).

**Parameters**

- `key: string`

**Returns void**

### setVerified

```ts
setVerified(verified): void
```

SetVerified sets the "verified" record field value (usually available with Auth collections).

**Parameters**

- `verified: boolean`

**Returns void**

### tableName

```ts
tableName(): string
```

TableName returns the table name associated with the current Record model.

**Returns string**

### tokenKey

```ts
tokenKey(): string
```

TokenKey returns the "tokenKey" record field value (usually available with Auth collections).

**Returns string**

### unhide

```ts
unhide(...fieldNames): core.Record
```

Unhide forces to unhide the specified fields from the public safe serialization of the record (even when the collection field itself is marked as hidden).

**Parameters**

- `Rest ...fieldNames: string[]`

**Returns core.Record**

### unmarshalJSON

```ts
unmarshalJSON(data): void
```

UnmarshalJSON implements the [json.Unmarshaler] interface.

**Parameters**

- `data: string | number[]`

**Returns void**

### unmarshalJSONField

```ts
unmarshalJSONField(key, result): void
```

Retrieves the "key" json field value and unmarshals it into "result".

Example

```
 result := struct {
     FirstName string `json:"first_name"`
 }{}
 err := m.UnmarshalJSONField("my_field_name", &result)
```

**Parameters**

- `key: string`
- `result: any`

**Returns void**

### validatePassword

```ts
validatePassword(password): boolean
```

ValidatePassword validates a plain password against the "password" record field.

Returns false if the password is incorrect.

**Parameters**

- `password: string`

**Returns boolean**

### verified

```ts
verified(): boolean
```

Verified returns the "verified" record field value (usually available with Auth collections).

**Returns boolean**

### withCustomData

```ts
withCustomData(state): core.Record
```

WithCustomData toggles the export/serialization of custom data fields (false by default).

**Parameters**

- `state: boolean`

**Returns core.Record**

## Properties

### id

```ts
id: string
```

Id is the primary key of the model. It is usually autogenerated by the parent model implementation.
