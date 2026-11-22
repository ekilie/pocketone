Source: https://pocketbase.io/jsvm/interfaces/PocketBase-1.html

# Interface PocketBase

**Hierarchy**

- [excludeHooks](https://pocketbase.io/jsvm/types/excludeHooks.html)<[pocketbase](https://pocketbase.io/jsvm/modules/pocketbase.html).[PocketBase](https://pocketbase.io/jsvm/interfaces/pocketbase.PocketBase.html)>

  - PocketBase

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

### execute

```ts
execute(): void
```

Execute initializes the application (if not already) and executes the pb.RootCmd with graceful shutdown support.

This method differs from pb.Start() by not registering the default system commands!

**Returns void**

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

### start

```ts
start(): void
```

Start starts the application, aka. registers the default system commands (serve, superuser, version) and executes pb.RootCmd.

**Returns void**

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

## Properties

### Optional rootCmd

```ts
rootCmd?: cobra.Command
```

RootCmd is the main console command
