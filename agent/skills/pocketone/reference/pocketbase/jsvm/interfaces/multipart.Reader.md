Source: https://pocketbase.io/jsvm/interfaces/multipart.Reader.html

# Interface Reader

Reader is an iterator over parts in a MIME multipart body. Reader's underlying parser consumes its input as needed. Seeking isn't supported.

**Hierarchy**

- Reader

## Methods

### nextPart

```ts
nextPart(): Part
```

NextPart returns the next part in the multipart or an error. When there are no more parts, the error [io.EOF] is returned.

As a special case, if the "Content-Transfer-Encoding" header has a value of "quoted-printable", that header is instead hidden and the body is transparently decoded during Read calls.

**Returns Part**

### nextRawPart

```ts
nextRawPart(): Part
```

NextRawPart returns the next part in the multipart or an error. When there are no more parts, the error [io.EOF] is returned.

Unlike [Reader.NextPart], it does not have special handling for "Content-Transfer-Encoding: quoted-printable".

**Returns Part**

### readForm

```ts
readForm(maxMemory): Form
```

ReadForm parses an entire multipart message whose parts have a Content-Disposition of "form-data". It stores up to maxMemory bytes + 10MB (reserved for non-file parts) in memory. File parts which can't be stored in memory will be stored on disk in temporary files. It returns [ErrMessageTooLarge] if all non-file parts can't be stored in memory.

**Parameters**

- `maxMemory: number`

**Returns Form**
