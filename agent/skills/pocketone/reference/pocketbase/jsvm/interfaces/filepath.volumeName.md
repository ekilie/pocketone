Source: https://pocketbase.io/jsvm/interfaces/filepath.volumeName.html

# Interface volumeName

**Hierarchy**

- volumeName

```ts
volumeName(path): string
```

VolumeName returns leading volume name. Given "C:\foo\bar" it returns "C:" on Windows. Given "\host\share\foo" it returns "\host\share". On other platforms it returns "".

**Parameters**

- `path: string`

**Returns string**
