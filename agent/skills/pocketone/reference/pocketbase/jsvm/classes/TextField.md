Source: https://pocketbase.io/jsvm/classes/TextField.html

# Class TextField

TextField defines "text" type field for storing any string value.

The respective zero record field value is empty string.

The following additional setter keys are available:

- "fieldName:autogenerate" - autogenerate field value if AutogeneratePattern is set. For example:

```
  record.Set("slug:autogenerate", "") // [random value]
  record.Set("slug:autogenerate", "abc-") // abc-[random value]
```

**Hierarchy**

- [core](https://pocketbase.io/jsvm/modules/core.html).[TextField](https://pocketbase.io/jsvm/interfaces/core.TextField.html)

  - TextField

**Implements**

- [core](https://pocketbase.io/jsvm/modules/core.html).[TextField](https://pocketbase.io/jsvm/interfaces/core.TextField.html)

## Constructors

### constructor

```ts
new TextField(data?): TextField
```

**Parameters**

- `Optional data: Partial<core.TextField>`

**Returns TextField**

## Methods

### columnType

```ts
columnType(app): string
```

ColumnType implements [Field.ColumnType] interface method.

**Parameters**

- `app: App`

**Returns string**

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

**Parameters**

- `ctx: context.Context`
- `app: App`
- `record: core.Record`
- `actionName: string`
- `actionFunc: (() => void)`

**Returns void**

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

### validatePlainValue

```ts
validatePlainValue(value): void
```

ValidatePlainValue validates the provided string against the field options.

**Parameters**

- `value: string`

**Returns void**

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

### autogeneratePattern

```ts
autogeneratePattern: string
```

AutogeneratePattern specifies an optional regex pattern that could be used to generate random string from it and set it automatically on record create if no explicit value is set or when the `:autogenerate` modifier is used.

Note: the generated value still needs to satisfy min, max, pattern (if set)

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

### max

```ts
max: number
```

Max specifies the maximum allowed string characters.

If zero, a default limit of 5000 is applied.

### min

```ts
min: number
```

Min specifies the minimum required string characters.

if zero value, no min limit is applied.

### name

```ts
name: string
```

Name (required) is the unique name of the field.

### pattern

```ts
pattern: string
```

Pattern specifies an optional regex pattern to match against the field value.

Leave it empty to skip the pattern check.

### presentable

```ts
presentable: boolean
```

Presentable hints the Dashboard UI to use the underlying field record value in the relation preview label.

### primaryKey

```ts
primaryKey: boolean
```

PrimaryKey will mark the field as primary key.

A single collection can have only 1 field marked as primary key.

### required

```ts
required: boolean
```

Required will require the field value to be non-empty string.

### system

```ts
system: boolean
```

System prevents the renaming and removal of the field.
