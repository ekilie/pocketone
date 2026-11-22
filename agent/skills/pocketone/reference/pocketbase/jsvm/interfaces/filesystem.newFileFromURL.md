Source: https://pocketbase.io/jsvm/interfaces/filesystem.newFileFromURL.html

# Interface newFileFromURL

**Hierarchy**

- newFileFromURL

```ts
newFileFromURL(ctx, url): filesystem.File
```

NewFileFromURL creates a new File from the provided url by downloading the resource and load it as BytesReader.

Example

```
 ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
 defer cancel()

 file, err := filesystem.NewFileFromURL(ctx, "https://example.com/image.png")
```

**Parameters**

- `ctx: context.Context`
- `url: string`

**Returns filesystem.File**
