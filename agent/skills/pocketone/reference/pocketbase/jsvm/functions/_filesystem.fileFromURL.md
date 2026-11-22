Source: https://pocketbase.io/jsvm/functions/_filesystem.fileFromURL.html

# Function fileFromURL

```ts
fileFromURL(url, secTimeout?): filesystem.File
```

fileFromURL creates a new File from the provided url by downloading the resource and creating a BytesReader.

Example:

```js
// with default max timeout of 120sec
const file1 = $filesystem.fileFromURL("https://...")

// with custom timeout of 15sec
const file2 = $filesystem.fileFromURL("https://...", 15)
```

**Parameters**

- `url: string`
- `Optional secTimeout: number`

**Returns filesystem.File**
