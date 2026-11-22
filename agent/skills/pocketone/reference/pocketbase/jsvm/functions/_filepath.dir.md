Source: https://pocketbase.io/jsvm/functions/_filepath.dir.html

# Function dir

```ts
dir(path): string
```

Dir returns all but the last element of path, typically the path's directory. After dropping the final element, Dir calls [Clean] on the path and trailing slashes are removed. If the path is empty, Dir returns ".". If the path consists entirely of separators, Dir returns a single separator. The returned path does not end in a separator unless it is the root directory.

On Windows, given a volume-only name such as "C:", Dir returns "C:.", the current directory on drive C. To obtain the drive's root "C:", use [VolumeName] combined with a separator.

**Parameters**

- `path: string`

**Returns string**
