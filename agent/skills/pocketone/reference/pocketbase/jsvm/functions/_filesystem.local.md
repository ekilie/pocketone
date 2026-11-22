Source: https://pocketbase.io/jsvm/functions/_filesystem.local.html

# Function local

```ts
local(dirPath): System
```

Initializes a new local-only filesystem instance (make sure to call `close()` after you are done working with it).

Most users should prefer `$app.newFilesystem()` which will construct a local or S3 filesystem based on the configured application settings.

**Parameters**

- `dirPath: string`

**Returns System**
