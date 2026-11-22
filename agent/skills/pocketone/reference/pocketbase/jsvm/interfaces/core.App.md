Source: https://pocketbase.io/jsvm/interfaces/core.App.html

# Interface App

App defines the main PocketBase app interface.

Note that the interface is not intended to be implemented manually by users and instead they should use core.BaseApp (either directly or as embedded field in a custom struct).

This interface exists to make testing easier and to allow users to create common and pluggable helpers and methods that doesn't rely on a specific wrapped app struct (hence the large interface size).

**Hierarchy**

- App

**Indexable**

```ts
[key: string]: any
```

## Methods

### auxConcurrentDB

```ts
auxConcurrentDB(): Builder
```

AuxConcurrentDB returns the concurrent app auxiliary.db builder instance.

This method is used mainly internally for executing db read operations in a concurrent/non-blocking manner.

Most users should use simply AuxDB() as it will automatically route the query execution to AuxConcurrentDB() or AuxNonconcurrentDB().

In a transaction the AuxConcurrentDB() and AuxNonconcurrentDB() refer to the same *dbx.TX instance.

**Returns Builder**

### auxDB

```ts
auxDB(): Builder
```

AuxDB returns the app auxiliary.db builder instance.

To minimize SQLITE_BUSY errors, it automatically routes the SELECT queries to the underlying concurrent db pool and everything else to the nonconcurrent one.

For more finer control over the used connections pools you can call directly AuxConcurrentDB() or AuxNonconcurrentDB().

**Returns Builder**

### auxDelete

```ts
auxDelete(model): void
```

AuxDelete deletes the specified model from the auxiliary database.

**Parameters**

- `model: Model`

**Returns void**

### auxDeleteWithContext

```ts
auxDeleteWithContext(ctx, model): void
```

AuxDeleteWithContext deletes the specified model from the auxiliary database (the context could be used to limit the query execution).

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**

### auxHasTable

```ts
auxHasTable(tableName): boolean
```

AuxHasTable checks if a table (or view) with the provided name exists (case insensitive) in the auxiliary.db.

**Parameters**

- `tableName: string`

**Returns boolean**

### auxModelQuery

```ts
auxModelQuery(model): SelectQuery
```

AuxModelQuery creates a new preconfigured select auxiliary.db query with preset SELECT, FROM and other common fields based on the provided model.

**Parameters**

- `model: Model`

**Returns SelectQuery**

### auxNonconcurrentDB

```ts
auxNonconcurrentDB(): Builder
```

AuxNonconcurrentDB returns the nonconcurrent app auxiliary.db builder instance.

The returned db instance is limited only to a single open connection, meaning that it can process only 1 db operation at a time (other queries queue up).

This method is used mainly internally and in the tests to execute write (save/delete) db operations as it helps with minimizing the SQLITE_BUSY errors.

Most users should use simply AuxDB() as it will automatically route the query execution to AuxConcurrentDB() or AuxNonconcurrentDB().

In a transaction the AuxConcurrentDB() and AuxNonconcurrentDB() refer to the same *dbx.TX instance.

**Returns Builder**

### auxRunInTransaction

```ts
auxRunInTransaction(fn): void
```

AuxRunInTransaction wraps fn into a transaction for the auxiliary app database.

It is safe to nest RunInTransaction calls as long as you use the callback's txApp.

**Parameters**

- `fn: ((txApp) => void)`

**Returns void**

### auxSave

```ts
auxSave(model): void
```

AuxSave validates and saves the specified model into the auxiliary app database.

If you don't want to run validations, use [App.AuxSaveNoValidate()].

**Parameters**

- `model: Model`

**Returns void**

### auxSaveNoValidate

```ts
auxSaveNoValidate(model): void
```

AuxSaveNoValidate saves the specified model into the auxiliary app database without performing validations.

If you want to also run validations before persisting, use [App.AuxSave()].

**Parameters**

- `model: Model`

**Returns void**

### auxSaveNoValidateWithContext

```ts
auxSaveNoValidateWithContext(ctx, model): void
```

AuxSaveNoValidateWithContext is the same as [App.AuxSaveNoValidate()] but allows specifying a context to limit the db execution.

If you want to also run validations before persisting, use [App.AuxSaveWithContext()].

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**

### auxSaveWithContext

```ts
auxSaveWithContext(ctx, model): void
```

AuxSaveWithContext is the same as [App.AuxSave()] but allows specifying a context to limit the db execution.

If you don't want to run validations, use [App.AuxSaveNoValidateWithContext()].

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**

### auxVacuum

```ts
auxVacuum(): void
```

AuxVacuum executes VACUUM on the auxiliary.db in order to reclaim unused auxiliary db disk space.

**Returns void**

### bootstrap

```ts
bootstrap(): void
```

Bootstrap initializes the application (aka. create data dir, open db connections, load settings, etc.).

It calls ClearBootstrap() if the application was already bootstrapped.

**Returns void**

### canAccessRecord

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

### clearBootstrap

```ts
clearBootstrap(): void
```

ClearBootstrap releases the initialized core app resources (closing db connections, stopping cron ticker, etc.).

This method is no-op if the application is not bootstrapped yet.

**Returns void**

### collectionQuery

```ts
collectionQuery(): SelectQuery
```

CollectionQuery returns a new Collection select query.

**Returns SelectQuery**

### concurrentDB

```ts
concurrentDB(): Builder
```

ConcurrentDB returns the concurrent app data.db builder instance.

This method is used mainly internally for executing db read operations in a concurrent/non-blocking manner.

Most users should use simply DB() as it will automatically route the query execution to ConcurrentDB() or NonconcurrentDB().

In a transaction the ConcurrentDB() and NonconcurrentDB() refer to the same *dbx.TX instance.

**Returns Builder**

### countRecords

```ts
countRecords(collectionModelOrIdentifier, ...exprs): number
```

CountRecords returns the total number of records in a collection.

**Parameters**

- `collectionModelOrIdentifier: any`
- `Rest ...exprs: Expression[]`

**Returns number**

### createBackup

```ts
createBackup(ctx, name): void
```

CreateBackup creates a new backup of the current app pb_data directory.

Backups can be stored on S3 if it is configured in app.Settings().Backups.

Please refer to the godoc of the specific CoreApp implementation for details on the backup procedures.

**Parameters**

- `ctx: context.Context`
- `name: string`

**Returns void**

### createViewFields

```ts
createViewFields(dangerousSelectQuery): core.FieldsList
```

CreateViewFields creates a new FieldsList from the provided select query.

There are some caveats:

- The select query must have an "id" column.
- Wildcard ("*") columns are not supported to avoid accidentally leaking sensitive data.

NB! Be aware that this method is vulnerable to SQL injection and the "dangerousSelectQuery" argument must come only from trusted input!

**Parameters**

- `dangerousSelectQuery: string`

**Returns core.FieldsList**

### cron

```ts
cron(): Cron
```

Cron returns the app cron instance.

**Returns Cron**

### dataDir

```ts
dataDir(): string
```

DataDir returns the app data directory path.

**Returns string**

### db

```ts
db(): Builder
```

DB returns the default app data.db builder instance.

To minimize SQLITE_BUSY errors, it automatically routes the SELECT queries to the underlying concurrent db pool and everything else to the nonconcurrent one.

For more finer control over the used connections pools you can call directly ConcurrentDB() or NonconcurrentDB().

**Returns Builder**

### delete

```ts
delete(model): void
```

Delete deletes the specified model from the regular app database.

**Parameters**

- `model: Model`

**Returns void**

### deleteAllAuthOriginsByRecord

```ts
deleteAllAuthOriginsByRecord(authRecord): void
```

DeleteAllAuthOriginsByRecord deletes all AuthOrigin models associated with the provided record.

Returns a combined error with the failed deletes.

**Parameters**

- `authRecord: core.Record`

**Returns void**

### deleteAllExternalAuthsByRecord

```ts
deleteAllExternalAuthsByRecord(authRecord): void
```

DeleteAllExternalAuthsByRecord deletes all ExternalAuth models associated with the provided record.

Returns a combined error with the failed deletes.

**Parameters**

- `authRecord: core.Record`

**Returns void**

### deleteAllMFAsByRecord

```ts
deleteAllMFAsByRecord(authRecord): void
```

DeleteAllMFAsByRecord deletes all MFA models associated with the provided record.

Returns a combined error with the failed deletes.

**Parameters**

- `authRecord: core.Record`

**Returns void**

### deleteAllOTPsByRecord

```ts
deleteAllOTPsByRecord(authRecord): void
```

DeleteAllOTPsByRecord deletes all OTP models associated with the provided record.

Returns a combined error with the failed deletes.

**Parameters**

- `authRecord: core.Record`

**Returns void**

### deleteExpiredMFAs

```ts
deleteExpiredMFAs(): void
```

DeleteExpiredMFAs deletes the expired MFAs for all auth collections.

**Returns void**

### deleteExpiredOTPs

```ts
deleteExpiredOTPs(): void
```

DeleteExpiredOTPs deletes the expired OTPs for all auth collections.

**Returns void**

### deleteOldLogs

```ts
deleteOldLogs(createdBefore): void
```

DeleteOldLogs delete all logs that are created before createdBefore.

**Parameters**

- `createdBefore: Time`

**Returns void**

### deleteTable

```ts
deleteTable(dangerousTableName): void
```

DeleteTable drops the specified table.

This method is a no-op if a table with the provided name doesn't exist.

NB! Be aware that this method is vulnerable to SQL injection and the "dangerousTableName" argument must come only from trusted input!

**Parameters**

- `dangerousTableName: string`

**Returns void**

### deleteView

```ts
deleteView(dangerousViewName): void
```

DeleteView drops the specified view name.

This method is a no-op if a view with the provided name doesn't exist.

NB! Be aware that this method is vulnerable to SQL injection and the "dangerousViewName" argument must come only from trusted input!

**Parameters**

- `dangerousViewName: string`

**Returns void**

### deleteWithContext

```ts
deleteWithContext(ctx, model): void
```

Delete deletes the specified model from the regular app database (the context could be used to limit the query execution).

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**

### dryRunView

```ts
dryRunView(dangerousSelectQuery, sampleSize): DryRunViewResult
```

DryRunView executes the provided query by creating a temporary view collection and returning a sample of the resulting query records (if valid).

The same caveats from CreateViewFields apply here too.

NB! Be aware that this method is vulnerable to SQL injection and the "dangerousSelectQuery" argument must come only from trusted input!

**Parameters**

- `dangerousSelectQuery: string`
- `sampleSize: number`

**Returns DryRunViewResult**

### encryptionEnv

```ts
encryptionEnv(): string
```

EncryptionEnv returns the name of the app secret env key (currently used primarily for optional settings encryption but this may change in the future).

**Returns string**

### expandRecord

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

### expandRecords

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

### findAllAuthOriginsByCollection

```ts
findAllAuthOriginsByCollection(collection): AuthOrigin[]
```

FindAllAuthOriginsByCollection returns all AuthOrigin models linked to the provided collection (in DESC order).

**Parameters**

- `collection: core.Collection`

**Returns AuthOrigin[]**

### findAllAuthOriginsByRecord

```ts
findAllAuthOriginsByRecord(authRecord): AuthOrigin[]
```

FindAllAuthOriginsByRecord returns all AuthOrigin models linked to the provided auth record (in DESC order).

**Parameters**

- `authRecord: core.Record`

**Returns AuthOrigin[]**

### findAllCollections

```ts
findAllCollections(...collectionTypes): core.Collection[]
```

FindCollections finds all collections by the given type(s).

If collectionTypes is not set, it returns all collections.

Example:

```
 app.FindAllCollections() // all collections
 app.FindAllCollections("auth", "view") // only auth and view collections
```

**Parameters**

- `Rest ...collectionTypes: string[]`

**Returns core.Collection[]**

### findAllExternalAuthsByCollection

```ts
findAllExternalAuthsByCollection(collection): ExternalAuth[]
```

FindAllExternalAuthsByCollection returns all ExternalAuth models linked to the provided auth collection.

**Parameters**

- `collection: core.Collection`

**Returns ExternalAuth[]**

### findAllExternalAuthsByRecord

```ts
findAllExternalAuthsByRecord(authRecord): ExternalAuth[]
```

FindAllExternalAuthsByRecord returns all ExternalAuth models linked to the provided auth record.

**Parameters**

- `authRecord: core.Record`

**Returns ExternalAuth[]**

### findAllMFAsByCollection

```ts
findAllMFAsByCollection(collection): MFA[]
```

FindAllMFAsByCollection returns all MFA models linked to the provided collection.

**Parameters**

- `collection: core.Collection`

**Returns MFA[]**

### findAllMFAsByRecord

```ts
findAllMFAsByRecord(authRecord): MFA[]
```

FindAllMFAsByRecord returns all MFA models linked to the provided auth record.

**Parameters**

- `authRecord: core.Record`

**Returns MFA[]**

### findAllOTPsByCollection

```ts
findAllOTPsByCollection(collection): OTP[]
```

FindAllOTPsByCollection returns all OTP models linked to the provided collection.

**Parameters**

- `collection: core.Collection`

**Returns OTP[]**

### findAllOTPsByRecord

```ts
findAllOTPsByRecord(authRecord): OTP[]
```

FindAllOTPsByRecord returns all OTP models linked to the provided auth record.

**Parameters**

- `authRecord: core.Record`

**Returns OTP[]**

### findAllRecords

```ts
findAllRecords(collectionModelOrIdentifier, ...exprs): core.Record[]
```

FindAllRecords finds all records matching specified db expressions.

Returns all collection records if no expression is provided.

Returns an empty slice if no records are found.

Example:

```
 // no extra expressions
 app.FindAllRecords("example")

 // with extra expressions
 expr1 := dbx.HashExp{"email": "test@example.com"}
 expr2 := dbx.NewExp("LOWER(username) = {:username}", dbx.Params{"username": "test"})
 app.FindAllRecords("example", expr1, expr2)
```

**Parameters**

- `collectionModelOrIdentifier: any`
- `Rest ...exprs: Expression[]`

**Returns core.Record[]**

### findAuthOriginById

```ts
findAuthOriginById(id): AuthOrigin
```

FindAuthOriginById returns a single AuthOrigin model by its id.

**Parameters**

- `id: string`

**Returns AuthOrigin**

### findAuthOriginByRecordAndFingerprint

```ts
findAuthOriginByRecordAndFingerprint(authRecord, fingerprint): AuthOrigin
```

FindAuthOriginByRecordAndFingerprint returns a single AuthOrigin model by its authRecord relation and fingerprint.

**Parameters**

- `authRecord: core.Record`
- `fingerprint: string`

**Returns AuthOrigin**

### findAuthRecordByEmail

```ts
findAuthRecordByEmail(collectionModelOrIdentifier, email): core.Record
```

FindAuthRecordByEmail finds the auth record associated with the provided email.

Returns an error if it is not an auth collection or the record is not found.

**Parameters**

- `collectionModelOrIdentifier: any`
- `email: string`

**Returns core.Record**

### findAuthRecordByToken

```ts
findAuthRecordByToken(token, ...validTypes): core.Record
```

FindAuthRecordByToken finds the auth record associated with the provided JWT (auth, file, verifyEmail, changeEmail, passwordReset types).

Optionally specify a list of validTypes to check tokens only from those types.

Returns an error if the JWT is invalid, expired or not associated to an auth collection record.

**Parameters**

- `token: string`
- `Rest ...validTypes: string[]`

**Returns core.Record**

### findCachedCollectionByNameOrId

```ts
findCachedCollectionByNameOrId(nameOrId): core.Collection
```

FindCachedCollectionByNameOrId is similar to [App.FindCollectionByNameOrId] but retrieves the Collection from the app cache instead of making a db call.

NB! This method is suitable for read-only Collection operations.

Returns [sql.ErrNoRows] if no Collection is found for consistency with the [App.FindCollectionByNameOrId] method.

If you plan making changes to the returned Collection model, use [App.FindCollectionByNameOrId] instead.

Caveats:

```
  - The returned Collection should be used only for read-only operations.
    Avoid directly modifying the returned cached Collection as it will affect
    the global cached value even if you don't persist the changes in the database!
  - If you are updating a Collection in a transaction and then call this method before commit,
    it'll return the cached Collection state and not the one from the uncommitted transaction.
  - The cache is automatically updated on collections db change (create/update/delete).
    To manually reload the cache you can call [App.ReloadCachedCollections]
```

**Parameters**

- `nameOrId: string`

**Returns core.Collection**

### findCachedCollectionReferences

```ts
findCachedCollectionReferences(collection, ...excludeIds): _TygojaDict
```

FindCachedCollectionReferences is similar to [App.FindCollectionReferences] but retrieves the Collection from the app cache instead of making a db call.

NB! This method is suitable for read-only Collection operations.

If you plan making changes to the returned Collection model, use [App.FindCollectionReferences] instead.

Caveats:

```
  - The returned Collection should be used only for read-only operations.
    Avoid directly modifying the returned cached Collection as it will affect
    the global cached value even if you don't persist the changes in the database!
  - If you are updating a Collection in a transaction and then call this method before commit,
    it'll return the cached Collection state and not the one from the uncommitted transaction.
  - The cache is automatically updated on collections db change (create/update/delete).
    To manually reload the cache you can call [App.ReloadCachedCollections].
```

**Parameters**

- `collection: core.Collection`
- `Rest ...excludeIds: string[]`

**Returns _TygojaDict**

### findCollectionByNameOrId

```ts
findCollectionByNameOrId(nameOrId): core.Collection
```

FindCollectionByNameOrId finds a single collection by its name (case insensitive) or id.s

**Parameters**

- `nameOrId: string`

**Returns core.Collection**

### findCollectionReferences

```ts
findCollectionReferences(collection, ...excludeIds): _TygojaDict
```

FindCollectionReferences returns information for all relation fields referencing the provided collection.

If the provided collection has reference to itself then it will be also included in the result. To exclude it, pass the collection id as the excludeIds argument.

**Parameters**

- `collection: core.Collection`
- `Rest ...excludeIds: string[]`

**Returns _TygojaDict**

### findFirstExternalAuthByExpr

```ts
findFirstExternalAuthByExpr(expr): ExternalAuth
```

FindFirstExternalAuthByExpr returns the first available (the most recent created) ExternalAuth model that satisfies the non-nil expression.

**Parameters**

- `expr: Expression`

**Returns ExternalAuth**

### findFirstRecordByData

```ts
findFirstRecordByData(collectionModelOrIdentifier, key, value): core.Record
```

FindFirstRecordByData returns the first found record matching the provided key-value pair.

**Parameters**

- `collectionModelOrIdentifier: any`
- `key: string`
- `value: any`

**Returns core.Record**

### findFirstRecordByFilter

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

### findLogById

```ts
findLogById(id): Log
```

FindLogById finds a single Log entry by its id.

**Parameters**

- `id: string`

**Returns Log**

### findMFAById

```ts
findMFAById(id): MFA
```

FindMFAById returns a single MFA model by its id.

**Parameters**

- `id: string`

**Returns MFA**

### findOTPById

```ts
findOTPById(id): OTP
```

FindOTPById returns a single OTP model by its id.

**Parameters**

- `id: string`

**Returns OTP**

### findRecordById

```ts
findRecordById(collectionModelOrIdentifier, recordId, ...optFilters): core.Record
```

FindRecordById finds the Record model by its id.

**Parameters**

- `collectionModelOrIdentifier: any`
- `recordId: string`
- `Rest ...optFilters: ((q) => void)[]`

**Returns core.Record**

### findRecordByViewFile

```ts
findRecordByViewFile(viewCollectionModelOrIdentifier, fileFieldName, filename): core.Record
```

FindRecordByViewFile returns the original Record of the provided view collection file.

**Parameters**

- `viewCollectionModelOrIdentifier: any`
- `fileFieldName: string`
- `filename: string`

**Returns core.Record**

### findRecordsByFilter

```ts
findRecordsByFilter(collectionModelOrIdentifier, filter, sort, limit, offset, ...params): core.Record[]
```

FindRecordsByFilter returns limit number of records matching the provided string filter.

NB! Use the last "params" argument to bind untrusted user variables!

The filter argument is optional and can be empty string to target all available records.

The sort argument is optional and can be empty string OR the same format used in the web APIs, ex. "-created,title".

If the limit argument is <= 0, no limit is applied to the query and all matching records are returned.

Returns an empty slice if no records are found.

Example:

```
 app.FindRecordsByFilter(
   "posts",
   "title ~ {:title} && visible = {:visible}",
   "-created",
   10,
   0,
   dbx.Params{"title": "lorem ipsum", "visible": true}
 )
```

**Parameters**

- `collectionModelOrIdentifier: any`
- `filter: string`
- `sort: string`
- `limit: number`
- `offset: number`
- `Rest ...params: Params[]`

**Returns core.Record[]**

### findRecordsByIds

```ts
findRecordsByIds(collectionModelOrIdentifier, recordIds, ...optFilters): core.Record[]
```

FindRecordsByIds finds all records by the specified ids. If no records are found, returns an empty slice.

**Parameters**

- `collectionModelOrIdentifier: any`
- `recordIds: string[]`
- `Rest ...optFilters: ((q) => void)[]`

**Returns core.Record[]**

### hasTable

```ts
hasTable(tableName): boolean
```

HasTable checks if a table (or view) with the provided name exists (case insensitive). in the data.db.

**Parameters**

- `tableName: string`

**Returns boolean**

### importCollections

```ts
importCollections(toImport, deleteMissing): void
```

ImportCollections imports the provided collections data in a single transaction.

For existing matching collections, the imported data is unmarshaled on top of the existing model.

NB! If deleteMissing is true, ALL NON-SYSTEM COLLECTIONS AND SCHEMA FIELDS, that are not present in the imported configuration, WILL BE DELETED (this includes their related records data).

**Parameters**

- `toImport: _TygojaDict[]`
- `deleteMissing: boolean`

**Returns void**

### importCollectionsByMarshaledJSON

```ts
importCollectionsByMarshaledJSON(rawSliceOfMaps, deleteMissing): void
```

ImportCollectionsByMarshaledJSON is the same as [ImportCollections] but accept marshaled json array as import data (usually used for the autogenerated snapshots).

**Parameters**

- `rawSliceOfMaps: string | number[]`
- `deleteMissing: boolean`

**Returns void**

### isBootstrapped

```ts
isBootstrapped(): boolean
```

IsBootstrapped checks if the application was initialized (aka. whether Bootstrap() was called).

**Returns boolean**

### isCollectionNameUnique

```ts
isCollectionNameUnique(name, ...excludeIds): boolean
```

IsCollectionNameUnique checks that there is no existing collection with the provided name (case insensitive!).

Note: case insensitive check because the name is used also as table name for the records.

**Parameters**

- `name: string`
- `Rest ...excludeIds: string[]`

**Returns boolean**

### isDev

```ts
isDev(): boolean
```

IsDev returns whether the app is in dev mode.

When enabled logs, executed sql statements, etc. are printed to the stderr.

**Returns boolean**

### isTransactional

```ts
isTransactional(): boolean
```

IsTransactional checks if the current app instance is part of a transaction.

**Returns boolean**

### logQuery

```ts
logQuery(): SelectQuery
```

LogQuery returns a new Log select query.

**Returns SelectQuery**

### logger

```ts
logger(): Logger
```

Logger returns the default app logger.

If the application is not bootstrapped yet, fallbacks to slog.Default().

**Returns Logger**

### logsStats

```ts
logsStats(expr): LogsStatsItem[]
```

LogsStatsItem returns hourly grouped logs statistics.

**Parameters**

- `expr: Expression`

**Returns LogsStatsItem[]**

### modelQuery

```ts
modelQuery(model): SelectQuery
```

ModelQuery creates a new preconfigured select data.db query with preset SELECT, FROM and other common fields based on the provided model.

**Parameters**

- `model: Model`

**Returns SelectQuery**

### newBackupsFilesystem

```ts
newBackupsFilesystem(): System
```

NewBackupsFilesystem creates a new local or S3 filesystem instance for managing app backups based on the current app settings.

NB! Make sure to call Close() on the returned result after you are done working with it.

**Returns System**

### newFilesystem

```ts
newFilesystem(): System
```

NewFilesystem creates a new local or S3 filesystem instance for managing regular app files (ex. record uploads) based on the current app settings.

NB! Make sure to call Close() on the returned result after you are done working with it.

**Returns System**

### newMailClient

```ts
newMailClient(): Mailer
```

NewMailClient creates and returns a new SMTP or Sendmail client based on the current app settings.

**Returns Mailer**

### nonconcurrentDB

```ts
nonconcurrentDB(): Builder
```

NonconcurrentDB returns the nonconcurrent app data.db builder instance.

The returned db instance is limited only to a single open connection, meaning that it can process only 1 db operation at a time (other queries queue up).

This method is used mainly internally and in the tests to execute write (save/delete) db operations as it helps with minimizing the SQLITE_BUSY errors.

Most users should use simply DB() as it will automatically route the query execution to ConcurrentDB() or NonconcurrentDB().

In a transaction the ConcurrentDB() and NonconcurrentDB() refer to the same *dbx.TX instance.

**Returns Builder**

### onBackupCreate

```ts
onBackupCreate(): Hook<BackupEvent>
```

OnBackupCreate hook is triggered on each [App.CreateBackup] call.

**Returns Hook<BackupEvent>**

### onBackupRestore

```ts
onBackupRestore(): Hook<BackupEvent>
```

OnBackupRestore hook is triggered before app backup restore (aka. [App.RestoreBackup] call).

Note that by default on success the application is restarted and the after state of the hook is ignored.

**Returns Hook<BackupEvent>**

### onBatchRequest

```ts
onBatchRequest(): Hook<BatchRequestEvent>
```

OnBatchRequest hook is triggered on each API batch request.

Could be used to additionally validate or modify the submitted batch requests.

**Returns Hook<BatchRequestEvent>**

### onBootstrap

```ts
onBootstrap(): Hook<BootstrapEvent>
```

OnBootstrap hook is triggered when initializing the main application resources (db, app settings, etc).

**Returns Hook<BootstrapEvent>**

### onBootstrapClear

```ts
onBootstrapClear(): Hook<BootstrapEvent>
```

OnBootstrapClear hook is triggered when clearing the main application resources (db connections, cron, logger, etc.)

It is usually invoked automatically right before app termination or when manually calling app.ClearBootstrap().

**Returns Hook<BootstrapEvent>**

### onCollectionAfterCreateError

```ts
onCollectionAfterCreateError(...tags): TaggedHook<CollectionErrorEvent>
```

OnCollectionAfterCreateError is a Collection proxy model hook of [OnModelAfterCreateError].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionErrorEvent>**

### onCollectionAfterCreateSuccess

```ts
onCollectionAfterCreateSuccess(...tags): TaggedHook<CollectionEvent>
```

OnCollectionAfterCreateSuccess is a Collection proxy model hook of [OnModelAfterCreateSuccess].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionAfterDeleteError

```ts
onCollectionAfterDeleteError(...tags): TaggedHook<CollectionErrorEvent>
```

OnCollectionAfterDeleteError is a Collection proxy model hook of [OnModelAfterDeleteError].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionErrorEvent>**

### onCollectionAfterDeleteSuccess

```ts
onCollectionAfterDeleteSuccess(...tags): TaggedHook<CollectionEvent>
```

OnCollectionAfterDeleteSuccess is a Collection proxy model hook of [OnModelAfterDeleteSuccess].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionAfterUpdateError

```ts
onCollectionAfterUpdateError(...tags): TaggedHook<CollectionErrorEvent>
```

OnCollectionAfterUpdateError is a Collection proxy model hook of [OnModelAfterUpdateError].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionErrorEvent>**

### onCollectionAfterUpdateSuccess

```ts
onCollectionAfterUpdateSuccess(...tags): TaggedHook<CollectionEvent>
```

OnCollectionAfterUpdateSuccess is a Collection proxy model hook of [OnModelAfterUpdateSuccess].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionCreate

```ts
onCollectionCreate(...tags): TaggedHook<CollectionEvent>
```

OnCollectionCreate is a Collection proxy model hook of [OnModelCreate].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionCreateExecute

```ts
onCollectionCreateExecute(...tags): TaggedHook<CollectionEvent>
```

OnCollectionCreateExecute is a Collection proxy model hook of [OnModelCreateExecute].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionCreateRequest

```ts
onCollectionCreateRequest(): Hook<CollectionRequestEvent>
```

OnCollectionCreateRequest hook is triggered on each API Collection create request.

Could be used to additionally validate the request data or implement completely different persistence behavior.

**Returns Hook<CollectionRequestEvent>**

### onCollectionDelete

```ts
onCollectionDelete(...tags): TaggedHook<CollectionEvent>
```

OnCollectionDelete is a Collection proxy model hook of [OnModelDelete].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionDeleteExecute

```ts
onCollectionDeleteExecute(...tags): TaggedHook<CollectionEvent>
```

OnCollectionDeleteExecute is a Collection proxy model hook of [OnModelDeleteExecute].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionDeleteRequest

```ts
onCollectionDeleteRequest(): Hook<CollectionRequestEvent>
```

OnCollectionDeleteRequest hook is triggered on each API Collection delete request.

Could be used to additionally validate the request data or implement completely different delete behavior.

**Returns Hook<CollectionRequestEvent>**

### onCollectionUpdate

```ts
onCollectionUpdate(...tags): TaggedHook<CollectionEvent>
```

OnCollectionUpdate is a Collection proxy model hook of [OnModelUpdate].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionUpdateExecute

```ts
onCollectionUpdateExecute(...tags): TaggedHook<CollectionEvent>
```

OnCollectionUpdateExecute is a Collection proxy model hook of [OnModelUpdateExecute].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionUpdateRequest

```ts
onCollectionUpdateRequest(): Hook<CollectionRequestEvent>
```

OnCollectionUpdateRequest hook is triggered on each API Collection update request.

Could be used to additionally validate the request data or implement completely different persistence behavior.

**Returns Hook<CollectionRequestEvent>**

### onCollectionValidate

```ts
onCollectionValidate(...tags): TaggedHook<CollectionEvent>
```

OnCollectionValidate is a Collection proxy model hook of [OnModelValidate].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<CollectionEvent>**

### onCollectionViewRequest

```ts
onCollectionViewRequest(): Hook<CollectionRequestEvent>
```

OnCollectionViewRequest hook is triggered on each API Collection view request.

Could be used to validate or modify the response before returning it to the client.

**Returns Hook<CollectionRequestEvent>**

### onCollectionsImportRequest

```ts
onCollectionsImportRequest(): Hook<CollectionsImportRequestEvent>
```

OnCollectionsBeforeImportRequest hook is triggered on each API collections import request.

Could be used to additionally validate the imported collections or to implement completely different import behavior.

**Returns Hook<CollectionsImportRequestEvent>**

### onCollectionsListRequest

```ts
onCollectionsListRequest(): Hook<CollectionsListRequestEvent>
```

OnCollectionsListRequest hook is triggered on each API Collections list request.

Could be used to validate or modify the response before returning it to the client.

**Returns Hook<CollectionsListRequestEvent>**

### onFileDownloadRequest

```ts
onFileDownloadRequest(...tags): TaggedHook<FileDownloadRequestEvent>
```

OnFileDownloadRequest hook is triggered before each API File download request.

Could be used to validate or modify the file response before returning it to the client.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<FileDownloadRequestEvent>**

### onFileTokenRequest

```ts
onFileTokenRequest(...tags): TaggedHook<FileTokenRequestEvent>
```

OnFileBeforeTokenRequest hook is triggered on each auth file token API request.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<FileTokenRequestEvent>**

### onMailerRecordAuthAlertSend

```ts
onMailerRecordAuthAlertSend(...tags): TaggedHook<MailerRecordEvent>
```

OnMailerRecordAuthAlertSend hook is triggered when sending a new device login auth alert email, allowing you to intercept and customize the email message that is being sent.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<MailerRecordEvent>**

### onMailerRecordEmailChangeSend

```ts
onMailerRecordEmailChangeSend(...tags): TaggedHook<MailerRecordEvent>
```

OnMailerRecordEmailChangeSend hook is triggered when sending a confirmation new address email to an auth record, allowing you to intercept and customize the email message that is being sent.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<MailerRecordEvent>**

### onMailerRecordOTPSend

```ts
onMailerRecordOTPSend(...tags): TaggedHook<MailerRecordEvent>
```

OnMailerRecordOTPSend hook is triggered when sending an OTP email to an auth record, allowing you to intercept and customize the email message that is being sent.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<MailerRecordEvent>**

### onMailerRecordPasswordResetSend

```ts
onMailerRecordPasswordResetSend(...tags): TaggedHook<MailerRecordEvent>
```

OnMailerBeforeRecordResetPasswordSend hook is triggered when sending a password reset email to an auth record, allowing you to intercept and customize the email message that is being sent.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<MailerRecordEvent>**

### onMailerRecordVerificationSend

```ts
onMailerRecordVerificationSend(...tags): TaggedHook<MailerRecordEvent>
```

OnMailerBeforeRecordVerificationSend hook is triggered when sending a verification email to an auth record, allowing you to intercept and customize the email message that is being sent.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<MailerRecordEvent>**

### onMailerSend

```ts
onMailerSend(): Hook<MailerEvent>
```

OnMailerSend hook is triggered every time when a new email is being sent using the [App.NewMailClient()] instance.

It allows intercepting the email message or to use a custom mailer client.

**Returns Hook<MailerEvent>**

### onModelAfterCreateError

```ts
onModelAfterCreateError(...tags): TaggedHook<ModelErrorEvent>
```

OnModelAfterCreateError is triggered after each failed Model DB create persistence.

Note that the execution of this hook is either immediate or delayed depending on the error:

```
  - "immediate" on App.Save() failure
  - "delayed" on transaction rollback
```

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelErrorEvent>**

### onModelAfterCreateSuccess

```ts
onModelAfterCreateSuccess(...tags): TaggedHook<ModelEvent>
```

OnModelAfterCreateSuccess is triggered after each successful Model DB create persistence.

Note that when a Model is persisted as part of a transaction, this hook is delayed and executed only AFTER the transaction has been committed. This hook is NOT triggered in case the transaction rollbacks (aka. when the model wasn't persisted).

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onModelAfterDeleteError

```ts
onModelAfterDeleteError(...tags): TaggedHook<ModelErrorEvent>
```

OnModelAfterDeleteError is triggered after each failed Model DB delete persistence.

Note that the execution of this hook is either immediate or delayed depending on the error:

```
  - "immediate" on App.Delete() failure
  - "delayed" on transaction rollback
```

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelErrorEvent>**

### onModelAfterDeleteSuccess

```ts
onModelAfterDeleteSuccess(...tags): TaggedHook<ModelEvent>
```

OnModelAfterDeleteSuccess is triggered after each successful Model DB delete persistence.

Note that when a Model is deleted as part of a transaction, this hook is delayed and executed only AFTER the transaction has been committed. This hook is NOT triggered in case the transaction rollbacks (aka. when the model delete wasn't persisted).

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onModelAfterUpdateError

```ts
onModelAfterUpdateError(...tags): TaggedHook<ModelErrorEvent>
```

OnModelAfterUpdateError is triggered after each failed Model DB update persistence.

Note that the execution of this hook is either immediate or delayed depending on the error:

```
  - "immediate" on App.Save() failure
  - "delayed" on transaction rollback
```

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelErrorEvent>**

### onModelAfterUpdateSuccess

```ts
onModelAfterUpdateSuccess(...tags): TaggedHook<ModelEvent>
```

OnModelAfterUpdateSuccess is triggered after each successful Model DB update persistence.

Note that when a Model is persisted as part of a transaction, this hook is delayed and executed only AFTER the transaction has been committed. This hook is NOT triggered in case the transaction rollbacks (aka. when the model changes weren't persisted).

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onModelCreate

```ts
onModelCreate(...tags): TaggedHook<ModelEvent>
```

OnModelCreate is triggered every time when a new model is being created (e.g. triggered by App.Save()).

Operations BEFORE the e.Next() execute before the model validation and the INSERT DB statement.

Operations AFTER the e.Next() execute after the model validation and the INSERT DB statement.

Note that successful execution doesn't guarantee that the model is persisted in the database since its wrapping transaction may not have been committed yet. If you want to listen to only the actual persisted events, you can bind to [OnModelAfterCreateSuccess] or [OnModelAfterCreateError] hooks.

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onModelCreateExecute

```ts
onModelCreateExecute(...tags): TaggedHook<ModelEvent>
```

OnModelCreateExecute is triggered after successful Model validation and right before the model INSERT DB statement execution.

Usually it is triggered as part of the App.Save() in the following firing order: OnModelCreate {

```
   -> OnModelValidate (skipped with App.SaveNoValidate())
   -> OnModelCreateExecute
```

}

Note that successful execution doesn't guarantee that the model is persisted in the database since its wrapping transaction may have been committed yet. If you want to listen to only the actual persisted events, you can bind to [OnModelAfterCreateSuccess] or [OnModelAfterCreateError] hooks.

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onModelDelete

```ts
onModelDelete(...tags): TaggedHook<ModelEvent>
```

OnModelDelete is triggered every time when a new model is being deleted (e.g. triggered by App.Delete()).

Note that successful execution doesn't guarantee that the model is deleted from the database since its wrapping transaction may not have been committed yet. If you want to listen to only the actual persisted deleted events, you can bind to [OnModelAfterDeleteSuccess] or [OnModelAfterDeleteError] hooks.

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onModelDeleteExecute

```ts
onModelDeleteExecute(...tags): TaggedHook<ModelEvent>
```

OnModelUpdateExecute is triggered right before the model DELETE DB statement execution.

Usually it is triggered as part of the App.Delete() in the following firing order: OnModelDelete {

```
   -> (internal delete checks)
   -> OnModelDeleteExecute
```

}

Note that successful execution doesn't guarantee that the model is deleted from the database since its wrapping transaction may not have been committed yet. If you want to listen to only the actual persisted deleted events, you can bind to [OnModelAfterDeleteSuccess] or [OnModelAfterDeleteError] hooks.

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onModelUpdate

```ts
onModelUpdate(...tags): TaggedHook<ModelEvent>
```

OnModelUpdate is triggered every time when a new model is being updated (e.g. triggered by App.Save()).

Operations BEFORE the e.Next() execute before the model validation and the UPDATE DB statement.

Operations AFTER the e.Next() execute after the model validation and the UPDATE DB statement.

Note that successful execution doesn't guarantee that the model is persisted in the database since its wrapping transaction may not have been committed yet. If you want to listen to only the actual persisted events, you can bind to [OnModelAfterUpdateSuccess] or [OnModelAfterUpdateError] hooks.

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onModelUpdateExecute

```ts
onModelUpdateExecute(...tags): TaggedHook<ModelEvent>
```

OnModelUpdateExecute is triggered after successful Model validation and right before the model UPDATE DB statement execution.

Usually it is triggered as part of the App.Save() in the following firing order: OnModelUpdate {

```
   -> OnModelValidate (skipped with App.SaveNoValidate())
   -> OnModelUpdateExecute
```

}

Note that successful execution doesn't guarantee that the model is persisted in the database since its wrapping transaction may have been committed yet. If you want to listen to only the actual persisted events, you can bind to [OnModelAfterUpdateSuccess] or [OnModelAfterUpdateError] hooks.

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onModelValidate

```ts
onModelValidate(...tags): TaggedHook<ModelEvent>
```

OnModelValidate is triggered every time when a model is being validated (e.g. triggered by App.Validate() or App.Save()).

For convenience, if you want to listen to only the Record models events without doing manual type assertion, you can attach to the OnRecord* proxy hooks.

If the optional "tags" list (Collection id/name, Model table name, etc.) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<ModelEvent>**

### onRealtimeConnectRequest

```ts
onRealtimeConnectRequest(): Hook<RealtimeConnectRequestEvent>
```

OnRealtimeConnectRequest hook is triggered when establishing the SSE client connection.

Any execution after e.Next() of a hook handler happens after the client disconnects.

**Returns Hook<RealtimeConnectRequestEvent>**

### onRealtimeMessageSend

```ts
onRealtimeMessageSend(): Hook<RealtimeMessageEvent>
```

OnRealtimeMessageSend hook is triggered when sending an SSE message to a client.

**Returns Hook<RealtimeMessageEvent>**

### onRealtimeSubscribeRequest

```ts
onRealtimeSubscribeRequest(): Hook<RealtimeSubscribeRequestEvent>
```

OnRealtimeSubscribeRequest hook is triggered when updating the client subscriptions, allowing you to further validate and modify the submitted change.

**Returns Hook<RealtimeSubscribeRequestEvent>**

### onRecordAfterCreateError

```ts
onRecordAfterCreateError(...tags): TaggedHook<RecordErrorEvent>
```

OnRecordAfterCreateError is a Record proxy model hook of [OnModelAfterCreateError].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordErrorEvent>**

### onRecordAfterCreateSuccess

```ts
onRecordAfterCreateSuccess(...tags): TaggedHook<RecordEvent>
```

OnRecordAfterCreateSuccess is a Record proxy model hook of [OnModelAfterCreateSuccess].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordAfterDeleteError

```ts
onRecordAfterDeleteError(...tags): TaggedHook<RecordErrorEvent>
```

OnRecordAfterDeleteError is a Record proxy model hook of [OnModelAfterDeleteError].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordErrorEvent>**

### onRecordAfterDeleteSuccess

```ts
onRecordAfterDeleteSuccess(...tags): TaggedHook<RecordEvent>
```

OnRecordAfterDeleteSuccess is a Record proxy model hook of [OnModelAfterDeleteSuccess].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordAfterUpdateError

```ts
onRecordAfterUpdateError(...tags): TaggedHook<RecordErrorEvent>
```

OnRecordAfterUpdateError is a Record proxy model hook of [OnModelAfterUpdateError].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordErrorEvent>**

### onRecordAfterUpdateSuccess

```ts
onRecordAfterUpdateSuccess(...tags): TaggedHook<RecordEvent>
```

OnRecordAfterUpdateSuccess is a Record proxy model hook of [OnModelAfterUpdateSuccess].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordAuthRefreshRequest

```ts
onRecordAuthRefreshRequest(...tags): TaggedHook<RecordAuthRefreshRequestEvent>
```

OnRecordAuthRefreshRequest hook is triggered on each Record auth refresh API request (right before generating a new auth token).

Could be used to additionally validate the request data or implement completely different auth refresh behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordAuthRefreshRequestEvent>**

### onRecordAuthRequest

```ts
onRecordAuthRequest(...tags): TaggedHook<RecordAuthRequestEvent>
```

OnRecordAuthRequest hook is triggered on each successful API record authentication request (sign-in, token refresh, etc.).

Could be used to additionally validate or modify the authenticated record data and token.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordAuthRequestEvent>**

### onRecordAuthWithOAuth2Request

```ts
onRecordAuthWithOAuth2Request(...tags): TaggedHook<RecordAuthWithOAuth2RequestEvent>
```

OnRecordAuthWithOAuth2Request hook is triggered on each Record OAuth2 sign-in/sign-up API request (after token exchange and before external provider linking).

If [RecordAuthWithOAuth2RequestEvent.Record] is not set, then the OAuth2 request will try to create a new auth Record.

To assign or link a different existing record model you can change the [RecordAuthWithOAuth2RequestEvent.Record] field.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordAuthWithOAuth2RequestEvent>**

### onRecordAuthWithOTPRequest

```ts
onRecordAuthWithOTPRequest(...tags): TaggedHook<RecordAuthWithOTPRequestEvent>
```

OnRecordAuthWithOTPRequest hook is triggered on each Record auth with OTP API request.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordAuthWithOTPRequestEvent>**

### onRecordAuthWithPasswordRequest

```ts
onRecordAuthWithPasswordRequest(...tags): TaggedHook<RecordAuthWithPasswordRequestEvent>
```

OnRecordAuthWithPasswordRequest hook is triggered on each Record auth with password API request.

[RecordAuthWithPasswordRequestEvent.Record] could be nil if no matching identity is found, allowing you to manually locate a different Record model (by reassigning [RecordAuthWithPasswordRequestEvent.Record]).

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordAuthWithPasswordRequestEvent>**

### onRecordConfirmEmailChangeRequest

```ts
onRecordConfirmEmailChangeRequest(...tags): TaggedHook<RecordConfirmEmailChangeRequestEvent>
```

OnRecordConfirmEmailChangeRequest hook is triggered on each Record confirm email change API request.

Could be used to additionally validate the request data or implement completely different persistence behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordConfirmEmailChangeRequestEvent>**

### onRecordConfirmPasswordResetRequest

```ts
onRecordConfirmPasswordResetRequest(...tags): TaggedHook<RecordConfirmPasswordResetRequestEvent>
```

OnRecordConfirmPasswordResetRequest hook is triggered on each Record confirm password reset API request.

Could be used to additionally validate the request data or implement completely different persistence behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordConfirmPasswordResetRequestEvent>**

### onRecordConfirmVerificationRequest

```ts
onRecordConfirmVerificationRequest(...tags): TaggedHook<RecordConfirmVerificationRequestEvent>
```

OnRecordConfirmVerificationRequest hook is triggered on each Record confirm verification API request.

Could be used to additionally validate the request data or implement completely different persistence behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordConfirmVerificationRequestEvent>**

### onRecordCreate

```ts
onRecordCreate(...tags): TaggedHook<RecordEvent>
```

OnRecordCreate is a Record proxy model hook of [OnModelCreate].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordCreateExecute

```ts
onRecordCreateExecute(...tags): TaggedHook<RecordEvent>
```

OnRecordCreateExecute is a Record proxy model hook of [OnModelCreateExecute].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordCreateRequest

```ts
onRecordCreateRequest(...tags): TaggedHook<RecordRequestEvent>
```

OnRecordCreateRequest hook is triggered on each API Record create request.

Could be used to additionally validate the request data or implement completely different persistence behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordRequestEvent>**

### onRecordDelete

```ts
onRecordDelete(...tags): TaggedHook<RecordEvent>
```

OnRecordDelete is a Record proxy model hook of [OnModelDelete].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordDeleteExecute

```ts
onRecordDeleteExecute(...tags): TaggedHook<RecordEvent>
```

OnRecordDeleteExecute is a Record proxy model hook of [OnModelDeleteExecute].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordDeleteRequest

```ts
onRecordDeleteRequest(...tags): TaggedHook<RecordRequestEvent>
```

OnRecordDeleteRequest hook is triggered on each API Record delete request.

Could be used to additionally validate the request data or implement completely different delete behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordRequestEvent>**

### onRecordEnrich

```ts
onRecordEnrich(...tags): TaggedHook<RecordEnrichEvent>
```

OnRecordEnrich is triggered every time when a record is enriched (as part of the builtin Record responses, during realtime message seriazation, or when [apis.EnrichRecord] is invoked).

It could be used for example to redact/hide or add computed temporary Record model props only for the specific request info. For example:

app.OnRecordEnrich("posts").BindFunc(func(e core.*RecordEnrichEvent) {

```
     // hide one or more fields
     e.Record.Hide("role")

     // add new custom field for registered users
     if e.RequestInfo.Auth != nil && e.RequestInfo.Auth.Collection().Name == "users" {
         e.Record.WithCustomData(true) // for security requires explicitly allowing it
         e.Record.Set("computedScore", e.Record.GetInt("score") * e.RequestInfo.Auth.GetInt("baseScore"))
     }

     return e.Next()
```

})

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEnrichEvent>**

### onRecordRequestEmailChangeRequest

```ts
onRecordRequestEmailChangeRequest(...tags): TaggedHook<RecordRequestEmailChangeRequestEvent>
```

OnRecordRequestEmailChangeRequest hook is triggered on each Record request email change API request.

Could be used to additionally validate the request data or implement completely different request email change behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordRequestEmailChangeRequestEvent>**

### onRecordRequestOTPRequest

```ts
onRecordRequestOTPRequest(...tags): TaggedHook<RecordCreateOTPRequestEvent>
```

OnRecordRequestOTPRequest hook is triggered on each Record request OTP API request.

[RecordCreateOTPRequestEvent.Record] could be nil if no matching identity is found, allowing you to manually create or locate a different Record model (by reassigning [RecordCreateOTPRequestEvent.Record]).

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordCreateOTPRequestEvent>**

### onRecordRequestPasswordResetRequest

```ts
onRecordRequestPasswordResetRequest(...tags): TaggedHook<RecordRequestPasswordResetRequestEvent>
```

OnRecordRequestPasswordResetRequest hook is triggered on each Record request password reset API request.

Could be used to additionally validate the request data or implement completely different password reset behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordRequestPasswordResetRequestEvent>**

### onRecordRequestVerificationRequest

```ts
onRecordRequestVerificationRequest(...tags): TaggedHook<RecordRequestVerificationRequestEvent>
```

OnRecordRequestVerificationRequest hook is triggered on each Record request verification API request.

Could be used to additionally validate the loaded request data or implement completely different verification behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordRequestVerificationRequestEvent>**

### onRecordUpdate

```ts
onRecordUpdate(...tags): TaggedHook<RecordEvent>
```

OnRecordUpdate is a Record proxy model hook of [OnModelUpdate].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordUpdateExecute

```ts
onRecordUpdateExecute(...tags): TaggedHook<RecordEvent>
```

OnRecordUpdateExecute is a Record proxy model hook of [OnModelUpdateExecute].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordUpdateRequest

```ts
onRecordUpdateRequest(...tags): TaggedHook<RecordRequestEvent>
```

OnRecordUpdateRequest hook is triggered on each API Record update request.

Could be used to additionally validate the request data or implement completely different persistence behavior.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordRequestEvent>**

### onRecordValidate

```ts
onRecordValidate(...tags): TaggedHook<RecordEvent>
```

OnRecordValidate is a Record proxy model hook of [OnModelValidate].

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordEvent>**

### onRecordViewRequest

```ts
onRecordViewRequest(...tags): TaggedHook<RecordRequestEvent>
```

OnRecordViewRequest hook is triggered on each API Record view request.

Could be used to validate or modify the response before returning it to the client.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordRequestEvent>**

### onRecordsListRequest

```ts
onRecordsListRequest(...tags): TaggedHook<RecordsListRequestEvent>
```

OnRecordsListRequest hook is triggered on each API Records list request.

Could be used to validate or modify the response before returning it to the client.

If the optional "tags" list (Collection ids or names) is specified, then all event handlers registered via the created hook will be triggered and called only if their event data origin matches the tags.

**Parameters**

- `Rest ...tags: string[]`

**Returns TaggedHook<RecordsListRequestEvent>**

### onServe

```ts
onServe(): Hook<ServeEvent>
```

OnServe hook is triggered when the app web server is started (after starting the TCP listener but before initializing the blocking serve task), allowing you to adjust its options and attach new routes or middlewares.

**Returns Hook<ServeEvent>**

### onSettingsListRequest

```ts
onSettingsListRequest(): Hook<SettingsListRequestEvent>
```

OnSettingsListRequest hook is triggered on each API Settings list request.

Could be used to validate or modify the response before returning it to the client.

**Returns Hook<SettingsListRequestEvent>**

### onSettingsReload

```ts
onSettingsReload(): Hook<SettingsReloadEvent>
```

OnSettingsReload hook is triggered every time when the App.Settings() is being replaced with a new state.

Calling App.Settings() after e.Next() returns the new state.

**Returns Hook<SettingsReloadEvent>**

### onSettingsUpdateRequest

```ts
onSettingsUpdateRequest(): Hook<SettingsUpdateRequestEvent>
```

OnSettingsUpdateRequest hook is triggered on each API Settings update request.

Could be used to additionally validate the request data or implement completely different persistence behavior.

**Returns Hook<SettingsUpdateRequestEvent>**

### onTerminate

```ts
onTerminate(): Hook<TerminateEvent>
```

OnTerminate hook is triggered when the app is in the process of being terminated (ex. on SIGTERM signal).

Note that the app could be terminated abruptly without awaiting the hook completion.

**Returns Hook<TerminateEvent>**

### recordQuery

```ts
recordQuery(collectionModelOrIdentifier): SelectQuery
```

RecordQuery returns a new Record select query from a collection model, id or name.

In case a collection id or name is provided and that collection doesn't actually exists, the generated query will be created with a cancelled context and will fail once an executor (Row(), One(), All(), etc.) is called.

**Parameters**

- `collectionModelOrIdentifier: any`

**Returns SelectQuery**

### reloadCachedCollections

```ts
reloadCachedCollections(): void
```

ReloadCachedCollections fetches all collections and caches them into the app store.

**Returns void**

### reloadSettings

```ts
reloadSettings(): void
```

ReloadSettings reinitializes and reloads the stored application settings.

**Returns void**

### resetBootstrapState

```ts
resetBootstrapState(): void
```

Deprecated: use ClearBootstrap().

**Returns void**

### restart

```ts
restart(): void
```

Restart restarts (aka. replaces) the current running application process.

NB! It relies on execve which is supported only on UNIX based systems.

**Returns void**

### restoreBackup

```ts
restoreBackup(ctx, name): void
```

RestoreBackup restores the backup with the specified name and restarts the current running application process.

The safely perform the restore it is recommended to have free disk space for at least 2x the size of the restored pb_data backup.

Please refer to the godoc of the specific CoreApp implementation for details on the restore procedures.

NB! This feature is experimental and currently is expected to work only on UNIX based systems.

**Parameters**

- `ctx: context.Context`
- `name: string`

**Returns void**

### runAllMigrations

```ts
runAllMigrations(): void
```

RunAllMigrations applies all system and app migrations (aka. from both [core.SystemMigrations] and [CoreAppMigrations]).

**Returns void**

### runAppMigrations

```ts
runAppMigrations(): void
```

RunAppMigrations applies all new migrations registered in the [CoreAppMigrations] list.

**Returns void**

### runInTransaction

```ts
runInTransaction(fn): void
```

RunInTransaction wraps fn into a transaction for the regular app database.

It is safe to nest RunInTransaction calls as long as you use the callback's txApp.

**Parameters**

- `fn: ((txApp) => void)`

**Returns void**

### runSystemMigrations

```ts
runSystemMigrations(): void
```

RunSystemMigrations applies all new migrations registered in the [core.SystemMigrations] list.

**Returns void**

### save

```ts
save(model): void
```

Save validates and saves the specified model into the regular app database.

If you don't want to run validations, use [App.SaveNoValidate()].

**Parameters**

- `model: Model`

**Returns void**

### saveNoValidate

```ts
saveNoValidate(model): void
```

SaveNoValidate saves the specified model into the regular app database without performing validations.

If you want to also run validations before persisting, use [App.Save()].

**Parameters**

- `model: Model`

**Returns void**

### saveNoValidateWithContext

```ts
saveNoValidateWithContext(ctx, model): void
```

SaveNoValidateWithContext is the same as [App.SaveNoValidate()] but allows specifying a context to limit the db execution.

If you want to also run validations before persisting, use [App.SaveWithContext()].

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**

### saveView

```ts
saveView(dangerousViewName, dangerousSelectQuery): void
```

SaveView creates (or updates already existing) persistent SQL view.

NB! Be aware that this method is vulnerable to SQL injection and its arguments must come only from trusted input!

**Parameters**

- `dangerousViewName: string`
- `dangerousSelectQuery: string`

**Returns void**

### saveWithContext

```ts
saveWithContext(ctx, model): void
```

SaveWithContext is the same as [App.Save()] but allows specifying a context to limit the db execution.

If you don't want to run validations, use [App.SaveNoValidateWithContext()].

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**

### settings

```ts
settings(): Settings
```

Settings returns the loaded app settings.

**Returns Settings**

### store

```ts
store(): Store<string, any>
```

Store returns the app runtime store.

**Returns Store<string, any>**

### subscriptionsBroker

```ts
subscriptionsBroker(): Broker
```

SubscriptionsBroker returns the app realtime subscriptions broker instance.

**Returns Broker**

### syncRecordTableSchema

```ts
syncRecordTableSchema(newCollection, oldCollection): void
```

SyncRecordTableSchema compares the two provided collections and applies the necessary related record table changes.

If oldCollection is null, then only newCollection is used to create the record table.

This method is automatically invoked as part of a collection create/update/delete operation.

**Parameters**

- `newCollection: core.Collection`
- `oldCollection: core.Collection`

**Returns void**

### tableColumns

```ts
tableColumns(tableName): string[]
```

TableColumns returns all column names of a single table by its name.

**Parameters**

- `tableName: string`

**Returns string[]**

### tableIndexes

```ts
tableIndexes(tableName): _TygojaDict
```

TableIndexes returns a name grouped map with all non empty index of the specified table.

Note: This method doesn't return an error on nonexisting table.

**Parameters**

- `tableName: string`

**Returns _TygojaDict**

### tableInfo

```ts
tableInfo(tableName): TableInfoRow[]
```

TableInfo returns the "table_info" pragma result for the specified table.

**Parameters**

- `tableName: string`

**Returns TableInfoRow[]**

### truncateCollection

```ts
truncateCollection(collection): void
```

TruncateCollection deletes all records associated with the provided collection.

The truncate operation is executed in a single transaction, aka. either everything is deleted or none.

Note that this method will also trigger the records related cascade and file delete actions.

**Parameters**

- `collection: core.Collection`

**Returns void**

### txInfo

```ts
txInfo(): TxAppInfo
```

TxInfo returns the transaction associated with the current app instance (if any).

Could be used if you want to execute indirectly a function after the related app transaction completes using `app.TxInfo().OnAfterFunc(callback)`.

**Returns TxAppInfo**

### unsafeWithoutHooks

```ts
unsafeWithoutHooks(): App
```

UnsafeWithoutHooks returns a shallow copy of the current app WITHOUT any registered hooks.

NB! Note that using the returned app instance may cause data integrity errors since the Record validations and data normalizations (including files uploads) rely on the app hooks to work.

**Returns App**

### vacuum

```ts
vacuum(): void
```

Vacuum executes VACUUM on the data.db in order to reclaim unused data db disk space.

**Returns void**

### validate

```ts
validate(model): void
```

Validate triggers the OnModelValidate hook for the specified model.

**Parameters**

- `model: Model`

**Returns void**

### validateWithContext

```ts
validateWithContext(ctx, model): void
```

ValidateWithContext is the same as Validate but allows specifying the ModelEvent context.

**Parameters**

- `ctx: context.Context`
- `model: Model`

**Returns void**
