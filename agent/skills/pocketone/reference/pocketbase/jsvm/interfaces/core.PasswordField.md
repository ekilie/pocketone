Source: https://pocketbase.io/jsvm/interfaces/core.PasswordField.html

# Interface PasswordField

PasswordField defines "password" type field for storing bcrypt hashed strings (usually used only internally for the "password" auth collection system field).

If you want to set a direct bcrypt hash as record field value you can use the SetRaw method, for example:

```
  // generates a bcrypt hash of "123456" and set it as field value
  // (record.GetString("password") returns the plain password until persisted, otherwise empty string)
  record.Set("password", "123456")

  // set directly a bcrypt hash of "123456" as field value
  // (record.GetString("password") returns empty string)
  record.SetRaw("password", "$2a$10$.5Elh8fgxypNUWhpUUr/xOa2sZm0VIaE0qWuGGl9otUfobb46T1Pq")
```

The following additional getter keys are available:

```
  - "fieldName:hash" - returns the bcrypt hash string of the record field value (if any). For example:
    record.GetString("password:hash")
```

**Hierarchy**

- PasswordField

  - [PasswordField](https://pocketbase.io/jsvm/classes/PasswordField.html)

**Implemented by**

- [PasswordField](https://pocketbase.io/jsvm/classes/PasswordField.html)

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

### cost

```ts
cost: number
```

Cost specifies the cost/weight/iteration/etc. bcrypt factor.

If zero, fallback to [bcrypt.DefaultCost].

If explicitly set, must be between [bcrypt.MinCost] and [bcrypt.MaxCost].

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

Max specifies an optional required field string length.

If zero, fallback to max 71 bytes.

### min

```ts
min: number
```

Min specifies an optional required field string length.

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

### Todo

remove

Presentable hints the Dashboard UI to use the underlying field record value in the relation preview label.

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
