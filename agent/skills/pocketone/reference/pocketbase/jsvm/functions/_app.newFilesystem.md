Source: https://pocketbase.io/jsvm/functions/_app.newFilesystem.html

# Function newFilesystem

```ts
newFilesystem(): System
```

NewFilesystem creates a new local or S3 filesystem instance for managing regular app files (ex. record uploads) based on the current app settings.

NB! Make sure to call Close() on the returned result after you are done working with it.

**Returns System**
