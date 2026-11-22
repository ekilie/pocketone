Source: https://pocketbase.io/jsvm/functions/_filepath.splitList.html

# Function splitList

```ts
splitList(path): string[]
```

SplitList splits a list of paths joined by the OS-specific [ListSeparator], usually found in PATH or GOPATH environment variables. Unlike strings.Split, SplitList returns an empty slice when passed an empty string.

**Parameters**

- `path: string`

**Returns string[]**
