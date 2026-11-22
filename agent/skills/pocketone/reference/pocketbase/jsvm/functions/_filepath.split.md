Source: https://pocketbase.io/jsvm/functions/_filepath.split.html

# Function split

```ts
split(path): [string, string]
```

Split splits path immediately following the final [Separator], separating it into a directory and file name component. If there is no Separator in path, Split returns an empty dir and file set to path. The returned values have the property that path = dir+file.

**Parameters**

- `path: string`

**Returns [string, string]**
