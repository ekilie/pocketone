Source: https://pocketbase.io/jsvm/interfaces/core.AutodateField.html

# Interface AutodateField

AutodateField defines an "autodate" type field, aka. field which datetime value could be auto set on record create/update.

This field is usually used for defining timestamp fields like "created" and "updated".

Requires either both or at least one of the OnCreate or OnUpdate options to be set.

**Hierarchy**

- AutodateField

  - [AutodateField](https://pocketbase.io/jsvm/classes/AutodateField.html)

**Implemented by**

- [AutodateField](https://pocketbase.io/jsvm/classes/AutodateField.html)

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

### name

```ts
name: string
```

Name (required) is the unique name of the field.

### onCreate

```ts
onCreate: boolean
```

OnCreate auto sets the current datetime as field value on record create.

### onUpdate

```ts
onUpdate: boolean
```

OnUpdate auto sets the current datetime as field value on record update.

### presentable

```ts
presentable: boolean
```

Presentable hints the Dashboard UI to use the underlying field record value in the relation preview label.

### system

```ts
system: boolean
```

System prevents the renaming and removal of the field.
