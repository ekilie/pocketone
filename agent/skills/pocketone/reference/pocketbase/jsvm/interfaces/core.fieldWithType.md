Source: https://pocketbase.io/jsvm/interfaces/core.fieldWithType.html

# Interface fieldWithType

**Hierarchy**

- [_sCuHQwy](https://pocketbase.io/jsvm/types/core._sCuHQwy.html)

  - fieldWithType

## Methods

### columnType

```ts
columnType(app): string
```

ColumnType returns the DB column definition of the field.

**Parameters**

- `app: App`

**Returns string**

### getHidden

```ts
getHidden(): boolean
```

GetHidden returns the field hidden flag state.

**Returns boolean**

### getId

```ts
getId(): string
```

GetId returns the field id.

**Returns string**

### getName

```ts
getName(): string
```

GetName returns the field name.

**Returns string**

### getSystem

```ts
getSystem(): boolean
```

GetSystem returns the field system flag state.

**Returns boolean**

### prepareValue

```ts
prepareValue(record, raw): any
```

PrepareValue returns a properly formatted field value based on the provided raw one.

This method is also called on record construction to initialize its default field value.

**Parameters**

- `record: core.Record`
- `raw: any`

**Returns any**

### setHidden

```ts
setHidden(hidden): void
```

SetHidden changes the field hidden flag state.

**Parameters**

- `hidden: boolean`

**Returns void**

### setId

```ts
setId(id): void
```

SetId changes the field id.

**Parameters**

- `id: string`

**Returns void**

### setName

```ts
setName(name): void
```

SetName changes the field name.

**Parameters**

- `name: string`

**Returns void**

### setSystem

```ts
setSystem(system): void
```

SetSystem changes the field system flag state.

**Parameters**

- `system: boolean`

**Returns void**

### unmarshalJSON

```ts
unmarshalJSON(data): void
```

**Parameters**

- `data: string | number[]`

**Returns void**

### validateSettings

```ts
validateSettings(ctx, app, collection): void
```

ValidateSettings validates the current field settings.

**Parameters**

- `ctx: context.Context`
- `app: App`
- `collection: core.Collection`

**Returns void**

### validateValue

```ts
validateValue(ctx, app, record): void
```

ValidateSettings validates the current field value associated with the provided record.

**Parameters**

- `ctx: context.Context`
- `app: App`
- `record: core.Record`

**Returns void**

## Properties

### type

```ts
type: string
```
