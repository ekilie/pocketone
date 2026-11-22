Source: https://pocketbase.io/jsvm/interfaces/core.BackupsConfig.html

# Interface BackupsConfig

**Hierarchy**

- BackupsConfig

## Methods

### validate

```ts
validate(): void
```

Validate makes BackupsConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### cron

```ts
cron: string
```

Cron is a cron expression to schedule auto backups, eg. "* * * * *".

Leave it empty to disable the auto backups functionality.

### cronMaxKeep

```ts
cronMaxKeep: number
```

CronMaxKeep is the max number of cron generated backups to keep before removing older entries.

This field works only when the cron config has valid cron expression.

### s3

```ts
s3: S3Config
```

S3 is an optional S3 storage config specifying where to store the app backups.
