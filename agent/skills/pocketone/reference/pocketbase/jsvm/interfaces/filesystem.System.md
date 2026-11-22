Source: https://pocketbase.io/jsvm/interfaces/filesystem.System.html

# Interface System

**Hierarchy**

- System

## Methods

### attributes

```ts
attributes(fileKey): Attributes
```

Attributes returns the attributes for the file with fileKey path.

If the file doesn't exist it returns ErrNotFound.

**Parameters**

- `fileKey: string`

**Returns Attributes**

### close

```ts
close(): void
```

Close releases any resources used for the related filesystem.

**Returns void**

### copy

```ts
copy(srcKey, dstKey): void
```

Copy copies the file stored at srcKey to dstKey.

If srcKey file doesn't exist, it returns ErrNotFound.

If dstKey file already exists, it is overwritten.

**Parameters**

- `srcKey: string`
- `dstKey: string`

**Returns void**

### createThumb

```ts
createThumb(originalKey, thumbKey, thumbSize): void
```

CreateThumb creates a new thumb image for the file at originalKey location. The new thumb file is stored at thumbKey location.

thumbSize is in the format:

- 0xH (eg. 0x100) - resize to H height preserving the aspect ratio
- Wx0 (eg. 300x0) - resize to W width preserving the aspect ratio
- WxH (eg. 300x100) - resize and crop to WxH viewbox (from center)
- WxHt (eg. 300x100t) - resize and crop to WxH viewbox (from top)
- WxHb (eg. 300x100b) - resize and crop to WxH viewbox (from bottom)
- WxHf (eg. 300x100f) - fit inside a WxH viewbox (without cropping)

**Parameters**

- `originalKey: string`
- `thumbKey: string`
- `thumbSize: string`

**Returns void**

### delete

```ts
delete(fileKey): void
```

Delete deletes stored file at fileKey location.

If the file doesn't exist returns ErrNotFound.

**Parameters**

- `fileKey: string`

**Returns void**

### deletePrefix

```ts
deletePrefix(prefix): Error[]
```

DeletePrefix deletes everything starting with the specified prefix.

The prefix could be subpath (ex. "/a/b/") or filename prefix (ex. "/a/b/file_").

**Parameters**

- `prefix: string`

**Returns Error[]**

### exists

```ts
exists(fileKey): boolean
```

Exists checks if file with fileKey path exists or not.

**Parameters**

- `fileKey: string`

**Returns boolean**

### getFile

```ts
getFile(fileKey): blob.Reader
```

Deprecated: Please use GetReader(fileKey) instead.

**Parameters**

- `fileKey: string`

**Returns blob.Reader**

### getReader

```ts
getReader(fileKey): blob.Reader
```

GetReader returns a file content reader for the given fileKey.

NB! Make sure to call Close() on the file after you are done working with it.

If the file doesn't exist returns ErrNotFound.

**Parameters**

- `fileKey: string`

**Returns blob.Reader**

### getReuploadableFile

```ts
getReuploadableFile(fileKey, preserveName): filesystem.File
```

GetReuploadableFile constructs a new reuploadable File value from the associated fileKey blob.Reader.

If preserveName is false then the returned File.Name will have a new randomly generated suffix, otherwise it will reuse the original one.

This method could be useful in case you want to clone an existing Record file and assign it to a new Record (e.g. in a Record duplicate action).

If you simply want to copy an existing file to a new location you could check the Copy(srcKey, dstKey) method.

**Parameters**

- `fileKey: string`
- `preserveName: boolean`

**Returns filesystem.File**

### isEmptyDir

```ts
isEmptyDir(dir): boolean
```

Checks if the provided dir prefix doesn't have any files.

A trailing slash will be appended to a non-empty dir string argument to ensure that the checked prefix is a "directory".

Returns "false" in case it has at least one file, otherwise - "true".

**Parameters**

- `dir: string`

**Returns boolean**

### list

```ts
list(prefix): ListObject[]
```

List returns a flat list with info for all files under the specified prefix.

**Parameters**

- `prefix: string`

**Returns ListObject[]**

### newWriter

```ts
newWriter(fileKey, opts): blob.Writer
```

NewWriter returns a new blob.Writer instance allowing direct file create from an io.Reader value.

If a file with the specified fileKey already exists, it will be replaced.

NB! Make sure to call `Close()` on the resulting writer after you are done working with it.

Note: If you have a bytes slice, [filesystem.File], or a multipart header value, you can check also the Upload* related methods as they are more user-friendly.

Example:

```
 content := strings.NewReader("Lorem ipsum dolor sit amet...")

 fsys, _ := filesystem.NewLocal("dir")
 defer fsys.Close()

 w, _ := fsys.NewWriter("example/file/key", nil)
 w.ReadFrom(content)
 w.Close()
```

**Parameters**

- `fileKey: string`
- `opts: WriterOptions`

**Returns blob.Writer**

### onDelete

```ts
onDelete(): Hook<DeleteEvent>
```

OnDelete is a low level hook that is triggered on every [System.Delete] call.

Note that the hook doesn't fire when a file is being overwritten by a new one, because in that case [System.Delete] is not invoked.

**Returns Hook<DeleteEvent>**

### onNewWriter

```ts
onNewWriter(): Hook<NewWriterEvent>
```

OnNewWriter is a low level hook that is triggered on every new writer initialization (aka. when attempting to create a new file with [system.NewWriter] or [system.Upload]).

Note that currently it doesn't trigger on [System.Copy] but this may change in future releases.

**Returns Hook<NewWriterEvent>**

### serve

```ts
serve(res, req, fileKey, name): void
```

Serve serves the file at fileKey location to an HTTP response.

If the `download` query parameter is used the file will be always served for download no matter of its type (aka. with "Content-Disposition: attachment").

Internally this method uses [http.ServeContent] so Range requests, If-Match, If-Unmodified-Since, etc. headers are handled transparently.

**Parameters**

- `res: ResponseWriter`
- `req: Request`
- `fileKey: string`
- `name: string`

**Returns void**

### setContext

```ts
setContext(ctx): void
```

SetContext assigns the specified context to the current filesystem.

**Parameters**

- `ctx: context.Context`

**Returns void**

### upload

```ts
upload(content, fileKey): void
```

Upload writes content into the fileKey location.

**Parameters**

- `content: string | number[]`
- `fileKey: string`

**Returns void**

### uploadFile

```ts
uploadFile(file, fileKey): void
```

UploadFile uploads the provided File to the fileKey location.

**Parameters**

- `file: filesystem.File`
- `fileKey: string`

**Returns void**

### uploadMultipart

```ts
uploadMultipart(fh, fileKey): void
```

UploadMultipart uploads the provided multipart file to the fileKey location.

**Parameters**

- `fh: FileHeader`
- `fileKey: string`

**Returns void**
