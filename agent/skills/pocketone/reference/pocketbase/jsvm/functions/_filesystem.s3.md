Source: https://pocketbase.io/jsvm/functions/_filesystem.s3.html

# Function s3

```ts
s3(bucketName, region, endpoint, accessKey, secretKey, s3ForcePathStyle): System
```

Initializes a new S3-only filesystem instance (make sure to call `close()` after you are done working with it).

Most users should prefer `$app.newFilesystem()` which will construct a local or S3 filesystem based on the configured application settings.

**Parameters**

- `bucketName: string`
- `region: string`
- `endpoint: string`
- `accessKey: string`
- `secretKey: string`
- `s3ForcePathStyle: boolean`

**Returns System**
