Source: https://pocketbase.io/jsvm/interfaces/core.SelectField.html

# Interface SelectField

SelectField defines "select" type field for storing single or multiple string values from a predefined list.

Requires the Values option to be set.

If MaxSelect is not set or <= 1, then the field value is expected to be a single Values element.

If MaxSelect is > 1, then the field value is expected to be a subset of Values slice.

The respective zero record field value is either empty string (single) or empty string slice (multiple).

---

The following additional setter keys are available:

```
  - "fieldName+" - append one or more values to the existing record one. For example:

    record.Set("roles+", []string{"new1", "new2"}) // []string{"old1", "old2", "new1", "new2"}

  - "+fieldName" - prepend one or more values to the existing record one. For example:

    record.Set("+roles", []string{"new1", "new2"}) // []string{"new1", "new2", "old1", "old2"}

  - "fieldName-" - subtract one or more values from the existing record one. For example:

    record.Set("roles-", "old1") // []string{"old2"}
```

**Hierarchy**

- SelectField

  - [SelectField](https://pocketbase.io/jsvm/classes/SelectField.html)

**Implemented by**

- [SelectField](https://pocketbase.io/jsvm/classes/SelectField.html)

## Methods

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

### isMultiple

```ts
isMultiple(): boolean
```

IsMultiple implements [MultiValuer] interface and checks whether the current field options support multiple values.

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

MaxSelect specifies the max allowed selected values.

For multiple select the value must be > 1, otherwise fallbacks to single (default).

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

Required will require the field value to be non-empty.

### system

```ts
system: boolean
```

System prevents the renaming and removal of the field.

### values

```ts
values: string[]
```

Values specifies the list of accepted values.
