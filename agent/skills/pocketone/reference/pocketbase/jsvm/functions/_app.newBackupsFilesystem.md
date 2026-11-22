Source: https://pocketbase.io/jsvm/functions/_app.newBackupsFilesystem.html

# Function newBackupsFilesystem

```ts
newBackupsFilesystem(): System
```

NewBackupsFilesystem creates a new local or S3 filesystem instance for managing app backups based on the current app settings.

NB! Make sure to call Close() on the returned result after you are done working with it.

**Returns System**
