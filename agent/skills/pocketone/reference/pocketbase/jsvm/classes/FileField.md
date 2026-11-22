Source: https://pocketbase.io/jsvm/classes/FileField.html

# Class FileField

FileField defines "file" type field for managing record file(s).

Only the file name is stored as part of the record value. New files (aka. files to upload) are expected to be of *filesystem.File.

If MaxSelect is not set or <= 1, then the field value is expected to be a single record id.

If MaxSelect is > 1, then the field value is expected to be a slice of record ids.

The respective zero record field value is either empty string (single) or empty string slice (multiple).

---

The following additional setter keys are available:

```
  - "fieldName+" - append one or more files to the existing record one. For example:

    // []string{"old1.txt", "old2.txt", "new1_ajkvass.txt", "new2_klhfnwd.txt"}
    record.Set("documents+", []*filesystem.File{new1, new2})

  - "+fieldName" - prepend one or more files to the existing record one. For example:

    // []string{"new1_ajkvass.txt", "new2_klhfnwd.txt", "old1.txt", "old2.txt",}
    record.Set("+documents", []*filesystem.File{new1, new2})

  - "fieldName-" - subtract/delete one or more files from the existing record one. For example:

    // []string{"old2.txt",}
    record.Set("documents-", "old1.txt")
```

**Hierarchy**

- [core](https://pocketbase.io/jsvm/modules/core.html).[FileField](https://pocketbase.io/jsvm/interfaces/core.FileField.html)

  - FileField

**Implements**

- [core](https://pocketbase.io/jsvm/modules/core.html).[FileField](https://pocketbase.io/jsvm/interfaces/core.FileField.html)

## Constructors

### constructor

```ts
new FileField(data?): FileField
```

**Parameters**

- `Optional data: Partial<core.FileField>`

**Returns FileField**

## Methods

### calculateMaxBodySize

```ts
calculateMaxBodySize(): number
```

CalculateMaxBodySize implements the [MaxBodySizeCalculator] interface.

**Returns number**

### columnType

```ts
columnType(app): string
```

ColumnType implements [Field.ColumnType] interface method.

**Parameters**

- `app: App`

**Returns string**

### driverValue

```ts
driverValue(record): any
```

DriverValue implements the [DriverValuer] interface.

**Parameters**

- `record: core.Record`

**Returns any**

### findGetter

```ts
findGetter(key): GetterFunc
```

FindGetter implements the [GetterFinder] interface.

**Parameters**

- `key: string`

**Returns GetterFunc**

### findSetter

```ts
findSetter(key): SetterFunc
```

FindSetter implements the [SetterFinder] interface.

**Parameters**

- `key: string`

**Returns SetterFunc**

### getHidden

```ts
getHidden(): boolean
```

GetHidden implements [Field.GetHidden] interface method.

**Returns boolean**

### getId

```ts
getId(): string
```

GetId implements [Field.GetId] interface method.

**Returns string**

### getName

```ts
getName(): string
```

GetName implements [Field.GetName] interface method.

**Returns string**

### getSystem

```ts
getSystem(): boolean
```

GetSystem implements [Field.GetSystem] interface method.

**Returns boolean**

### intercept

```ts
intercept(ctx, app, record, actionName, actionFunc): void
```

Intercept implements the [RecordInterceptor] interface.

note: files delete after records deletion is handled globally by the app FileManager hook

**Parameters**

- `ctx: context.Context`
- `app: App`
- `record: core.Record`
- `actionName: string`
- `actionFunc: (() => void)`

**Returns void**

### isMultiple

```ts
isMultiple(): boolean
```

IsMultiple implements MultiValuer interface and checks whether the current field options support multiple values.

**Returns boolean**

### prepareValue

```ts
prepareValue(record, raw): any
```

PrepareValue implements [Field.PrepareValue] interface method.

**Parameters**

- `record: core.Record`
- `raw: any`

**Returns any**

### setHidden

```ts
setHidden(hidden): void
```

SetHidden implements [Field.SetHidden] interface method.

**Parameters**

- `hidden: boolean`

**Returns void**

### setId

```ts
setId(id): void
```

SetId implements [Field.SetId] interface method.

**Parameters**

- `id: string`

**Returns void**

### setName

```ts
setName(name): void
```

SetName implements [Field.SetName] interface method.

**Parameters**

- `name: string`

**Returns void**

### setSystem

```ts
setSystem(system): void
```

SetSystem implements [Field.SetSystem] interface method.

**Parameters**

- `system: boolean`

**Returns void**

### type

```ts
type(): string
```

Type implements [Field.Type] interface method.

**Returns string**

### validateSettings

```ts
validateSettings(ctx, app, collection): void
```

ValidateSettings implements [Field.ValidateSettings] interface method.

**Parameters**

- `ctx: context.Context`
- `app: App`
- `collection: core.Collection`

**Returns void**

### validateValue

```ts
validateValue(ctx, app, record): void
```

ValidateValue implements [Field.ValidateValue] interface method.

**Parameters**

- `ctx: context.Context`
- `app: App`
- `record: core.Record`

**Returns void**

## Properties

### help

```ts
help: string
```

Help is an extra text explaining what the field is about. It is usually shown in Dashboard UI under the field input.

### hidden

```ts
hidden: boolean
```

Hidden hides the field from the API response.

### id

```ts
id: string
```

Id is the unique stable field identifier.

It is automatically generated from the name when adding to a collection FieldsList.

### maxSelect

```ts
maxSelect: number
```

MaxSelect specifies the max allowed files.

For multiple files the value must be > 1, otherwise fallbacks to single (default).

### maxSize

```ts
maxSize: number
```

MaxSize specifies the maximum size of a single uploaded file (in bytes and up to 2^53-1).

If zero, a default limit of 5MB is applied.

### mimeTypes

```ts
mimeTypes: string[]
```

MimeTypes specifies an optional list of the allowed file mime types.

Leave it empty to disable the validator.

### name

```ts
name: string
```

Name (required) is the unique name of the field.

### presentable

```ts
presentable: boolean
```

Presentable hints the Dashboard UI to use the underlying field record value in the relation preview label.

### protected

```ts
protected: boolean
```

Protected will require the users to provide a special file token to access the file.

Note that by default all files are publicly accessible.

For the majority of the cases this is fine because by default all file names have random part appended to their name which need to be known by the user before accessing the file.

### required

```ts
required: boolean
```

Required will require the field value to have at least one file.

### system

```ts
system: boolean
```

System prevents the renaming and removal of the field.

### thumbs

```ts
thumbs: string[]
```

Thumbs specifies an optional list of the supported thumbs for image based files.

Each entry must be in one of the following formats:

```
  - WxH  (eg. 100x300) - crop to WxH viewbox (from center)
  - WxHt (eg. 100x300t) - crop to WxH viewbox (from top)
  - WxHb (eg. 100x300b) - crop to WxH viewbox (from bottom)
  - WxHf (eg. 100x300f) - fit inside a WxH viewbox (without cropping)
  - 0xH  (eg. 0x300)    - resize to H height preserving the aspect ratio
  - Wx0  (eg. 100x0)    - resize to W width preserving the aspect ratio
```
