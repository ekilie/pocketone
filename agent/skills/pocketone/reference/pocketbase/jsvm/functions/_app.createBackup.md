Source: https://pocketbase.io/jsvm/functions/_app.createBackup.html

# Function createBackup

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
