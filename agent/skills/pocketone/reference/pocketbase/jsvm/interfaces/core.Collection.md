Source: https://pocketbase.io/jsvm/interfaces/core.Collection.html

# Interface Collection

Collection defines the table, fields and various options related to a set of records.

**Hierarchy**

- [_swEoIdC](https://pocketbase.io/jsvm/types/core._swEoIdC.html)

  - Collection

    - [Collection](https://pocketbase.io/jsvm/classes/Collection.html)

**Implemented by**

- [Collection](https://pocketbase.io/jsvm/classes/Collection.html)

## Methods

### addIndex

```ts
addIndex(name, unique, columnsExpr, optWhereExpr): void
```

AddIndex adds a new index into the current collection.

If the collection has an existing index matching the new name it will be replaced with the new one.

**Parameters**

- `name: string`
- `unique: boolean`
- `columnsExpr: string`
- `optWhereExpr: string`

**Returns void**

### baseFilesPath

```ts
baseFilesPath(): string
```

BaseFilesPath returns the storage dir path used by the collection.

**Returns string**

### dbExport

```ts
dbExport(app): _TygojaDict
```

DBExport prepares and exports the current collection data for db persistence.

**Parameters**

- `app: App`

**Returns _TygojaDict**

### getIndex

```ts
getIndex(name): string
```

GetIndex returns s single Collection index expression by its name.

**Parameters**

- `name: string`

**Returns string**

### integrityChecks

```ts
integrityChecks(enable): void
```

IntegrityChecks toggles the current collection integrity checks (ex. checking references on delete).

**Parameters**

- `enable: boolean`

**Returns void**

### isAuth

```ts
isAuth(): boolean
```

IsAuth checks if the current collection has "auth" type.

**Returns boolean**

### isBase

```ts
isBase(): boolean
```

IsBase checks if the current collection has "base" type.

**Returns boolean**

### isNew

```ts
isNew(): boolean
```

IsNew indicates what type of db query (insert or update) should be used with the model instance.

**Returns boolean**

### isView

```ts
isView(): boolean
```

IsView checks if the current collection has "view" type.

**Returns boolean**

### lastSavedPK

```ts
lastSavedPK(): any
```

LastSavedPK returns the last saved primary key of the model.

Its value is updated to the latest PK value after MarkAsNotNew() or PostScan() calls.

**Returns any**

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

Note that non-type related fields are ignored from the serialization (ex. for "view" collections the "auth" fields are skipped).

**Returns string | number[]**

### pk

```ts
pk(): any
```

**Returns any**

### postScan

```ts
postScan(): void
```

PostScan implements the [dbx.PostScanner] interface to auto unmarshal the raw serialized options into the concrete type specific fields.

**Returns void**

### removeIndex

```ts
removeIndex(name): void
```

RemoveIndex removes a single index with the specified name from the current collection.

**Parameters**

- `name: string`

**Returns void**

### string

```ts
string(): string
```

String returns a string representation of the current collection.

**Returns string**

### tableName

```ts
tableName(): string
```

TableName returns the Collection model SQL table name.

**Returns string**

### unmarshalJSON

```ts
unmarshalJSON(b): void
```

UnmarshalJSON implements the [json.Unmarshaler] interface.

For new/"blank" Collection models it replaces the model with a factory instance and then unmarshal the provided data one on top of it.

**Parameters**

- `b: string | number[]`

**Returns void**

## Properties

### authAlert

```ts
authAlert: AuthAlertConfig
```

AuthAlert defines options related to the auth alerts on new device login.

### Optional authRule

```ts
authRule?: string
```

AuthRule could be used to specify additional record constraints applied after record authentication and right before returning the auth token response to the client.

For example, to allow only verified users you could set it to "verified = true".

Set it to empty string to allow any Auth collection record to authenticate.

Set it to nil to disallow authentication altogether for the collection (that includes password, OAuth2, etc.).

### authToken

```ts
authToken: TokenConfig
```

## Various token configurations

### confirmEmailChangeTemplate

```ts
confirmEmailChangeTemplate: EmailTemplate
```

### Optional createRule

```ts
createRule?: string
```

### created

```ts
created: types.DateTime
```

### Optional deleteRule

```ts
deleteRule?: string
```

### emailChangeToken

```ts
emailChangeToken: TokenConfig
```

### fields

```ts
fields: core.FieldsList
```

### fileToken

```ts
fileToken: TokenConfig
```

### id

```ts
id: string
```

Id is the primary key of the model. It is usually autogenerated by the parent model implementation.

### indexes

```ts
indexes: JSONArray<string>
```

### Optional listRule

```ts
listRule?: string
```

### Optional manageRule

```ts
manageRule?: string
```

ManageRule gives admin-like permissions to allow fully managing the auth record(s), eg. changing the password without requiring to enter the old one, directly updating the verified state and email, etc.

This rule is executed in addition to the Create and Update API rules.

### mfa

```ts
mfa: MFAConfig
```

MFA defines options related to the Multi-factor authentication (MFA).

### name

```ts
name: string
```

### oauth2

```ts
oauth2: OAuth2Config
```

OAuth2 specifies whether OAuth2 auth is enabled for the collection and which OAuth2 providers are allowed.

### otp

```ts
otp: OTPConfig
```

OTP defines options related to the One-time password authentication (OTP).

### passwordAuth

```ts
passwordAuth: PasswordAuthConfig
```

PasswordAuth defines options related to the collection password authentication.

### passwordResetToken

```ts
passwordResetToken: TokenConfig
```

### rawOptions

```ts
rawOptions: JSONRaw
```

RawOptions represents the raw serialized collection option loaded from the DB. NB! This field shouldn't be modified manually. It is automatically updated with the collection type specific option before save.

### resetPasswordTemplate

```ts
resetPasswordTemplate: EmailTemplate
```

### system

```ts
system: boolean
```

System prevents the collection rename, deletion and rules change. It is used primarily for internal purposes for collections like "_superusers", "_externalAuths", etc.

### type

```ts
type: string
```

### Optional updateRule

```ts
updateRule?: string
```

### updated

```ts
updated: types.DateTime
```

### verificationTemplate

```ts
verificationTemplate: EmailTemplate
```

## Default email templates

### verificationToken

```ts
verificationToken: TokenConfig
```

### viewQuery

```ts
viewQuery: string
```

### Optional viewRule

```ts
viewRule?: string
```
