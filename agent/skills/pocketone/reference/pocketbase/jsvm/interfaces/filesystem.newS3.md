Source: https://pocketbase.io/jsvm/interfaces/filesystem.newS3.html

# Interface newS3

**Hierarchy**

- newS3

```ts
newS3(bucketName, region, endpoint, accessKey, secretKey, s3ForcePathStyle): System
```

NewS3 initializes a new S3 filesystem instance.

NB! Make sure to call `Close()` after you are done working with it.

**Parameters**

- `bucketName: string`
- `region: string`
- `endpoint: string`
- `accessKey: string`
- `secretKey: string`
- `s3ForcePathStyle: boolean`

**Returns System**
