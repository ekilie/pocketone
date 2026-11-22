Source: https://pocketbase.io/jsvm/functions/_app.restoreBackup.html

# Function restoreBackup

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
