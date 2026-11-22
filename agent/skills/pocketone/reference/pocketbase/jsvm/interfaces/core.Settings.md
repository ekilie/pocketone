Source: https://pocketbase.io/jsvm/interfaces/core.Settings.html

# Interface Settings

Settings defines the PocketBase app settings.

**Hierarchy**

- [_smcDExM](https://pocketbase.io/jsvm/types/core._smcDExM.html)

  - Settings

## Methods

### clone

```ts
clone(): Settings
```

Clone creates a new deep copy of the current settings.

**Returns Settings**

### dbExport

```ts
dbExport(app): _TygojaDict
```

DBExport prepares and exports the current settings for db persistence.

**Parameters**

- `app: App`

**Returns _TygojaDict**

### isNew

```ts
isNew(): boolean
```

IsNew implements [Model.IsNew] interface method.

**Returns boolean**

### lastSavedPK

```ts
lastSavedPK(): any
```

PK implements [Model.LastSavedPK] interface method.

**Returns any**

### markAsNew

```ts
markAsNew(): void
```

MarkAsNew implements [Model.MarkAsNew] interface method.

**Returns void**

### markAsNotNew

```ts
markAsNotNew(): void
```

MarkAsNew implements [Model.MarkAsNotNew] interface method.

**Returns void**

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements the [json.Marshaler] interface.

Note that sensitive fields (S3 secret, SMTP password, etc.) are excluded.

**Returns string | number[]**

### merge

```ts
merge(other): void
```

Merge merges the "other" settings into the current one.

**Parameters**

- `other: Settings`

**Returns void**

### pk

```ts
pk(): any
```

PK implements [Model.PK] interface method.

**Returns any**

### postScan

```ts
postScan(): void
```

PostScan implements [Model.PostScan] interface method.

**Returns void**

### postValidate

```ts
postValidate(ctx, app): void
```

PostValidate implements the [PostValidator] interface and defines the Settings model validations.

**Parameters**

- `ctx: context.Context`
- `app: App`

**Returns void**

### string

```ts
string(): string
```

String returns a serialized string representation of the current settings.

**Returns string**

### tableName

```ts
tableName(): string
```

TableName implements [Model.TableName] interface method.

**Returns string**

## Properties

### backups

```ts
backups: BackupsConfig
```

### batch

```ts
batch: BatchConfig
```

### logs

```ts
logs: LogsConfig
```

### meta

```ts
meta: MetaConfig
```

### rateLimits

```ts
rateLimits: RateLimitsConfig
```

### s3

```ts
s3: S3Config
```

### smtp

```ts
smtp: SMTPConfig
```

### superuserIPs

```ts
superuserIPs: string[]
```

SuperuserIPs defines an optional list of the superuser allowed individual IPs and subnets (in CIDR notation).

### trustedProxy

```ts
trustedProxy: TrustedProxyConfig
```
