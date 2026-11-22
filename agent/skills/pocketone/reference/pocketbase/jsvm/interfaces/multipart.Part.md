Source: https://pocketbase.io/jsvm/interfaces/multipart.Part.html

# Interface Part

A Part represents a single part in a multipart body.

**Hierarchy**

- Part

## Methods

### close

```ts
close(): void
```

**Returns void**

### fileName

```ts
fileName(): string
```

FileName returns the filename parameter of the [Part]'s Content-Disposition header. If not empty, the filename is passed through filepath.Base (which is platform dependent) before being returned.

**Returns string**

### formName

```ts
formName(): string
```

FormName returns the name parameter if p has a Content-Disposition of type "form-data". Otherwise it returns the empty string.

**Returns string**

### read

```ts
read(d): number
```

Read reads the body of a part, after its headers and before the next part (if any) begins.

**Parameters**

- `d: string | number[]`

**Returns number**

## Properties

### header

```ts
header: MIMEHeader
```

The headers of the body, if any, with the keys canonicalized in the same fashion that the Go http.Request headers are. For example, "foo-bar" changes case to "Foo-Bar"
