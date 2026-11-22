Source: https://pocketbase.io/jsvm/interfaces/core.JSONField.html

# Interface JSONField

JSONField defines "json" type field for storing any serialized JSON value.

The respective zero record field value is the zero [types.JSONRaw].

**Hierarchy**

- JSONField

  - [JSONField](https://pocketbase.io/jsvm/classes/JSONField.html)

**Implemented by**

- [JSONField](https://pocketbase.io/jsvm/classes/JSONField.html)

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

### maxSize

```ts
maxSize: number
```

MaxSize specifies the maximum size of the allowed field value (in bytes and up to 2^53-1).

If zero, a default limit of 1MB is applied.

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

### required

```ts
required: boolean
```

Required will require the field value to be non-empty JSON value (aka. not "null", `""`, "[]", "{}").

### system

```ts
system: boolean
```

System prevents the renaming and removal of the field.
