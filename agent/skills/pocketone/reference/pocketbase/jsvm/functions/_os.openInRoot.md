Source: https://pocketbase.io/jsvm/functions/_os.openInRoot.html

# Function openInRoot

```ts
openInRoot(dir, name): os.File
```

OpenInRoot opens the file name in the directory dir. It is equivalent to OpenRoot(dir) followed by opening the file in the root.

OpenInRoot returns an error if any component of the name references a location outside of dir.

See [Root] for details and limitations.

**Parameters**

- `dir: string`
- `name: string`

**Returns os.File**
