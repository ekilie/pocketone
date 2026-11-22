Source: https://pocketbase.io/jsvm/functions/_os.removeAll.html

# Function removeAll

```ts
removeAll(path): void
```

RemoveAll removes path and any children it contains. It removes everything it can but returns the first error it encounters. If the path does not exist, RemoveAll returns nil (no error). If there is an error, it will be of type [*PathError].

**Parameters**

- `path: string`

**Returns void**
