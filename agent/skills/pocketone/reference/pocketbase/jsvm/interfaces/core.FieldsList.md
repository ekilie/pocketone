Source: https://pocketbase.io/jsvm/interfaces/core.FieldsList.html

# Interface FieldsList

FieldsList defines a Collection slice of fields.

**Hierarchy**

- Array<[core](https://pocketbase.io/jsvm/modules/core.html).[Field](https://pocketbase.io/jsvm/interfaces/core.Field.html)>

  - FieldsList

    - [FieldsList](https://pocketbase.io/jsvm/classes/FieldsList.html)

**Implemented by**

- [FieldsList](https://pocketbase.io/jsvm/classes/FieldsList.html)

## Methods

### add

```ts
add(...fields): void
```

Add adds one or more fields to the current list.

By default this method will try to REPLACE existing fields with the new ones by their id or by their name if the new field doesn't have an explicit id.

If no matching existing field is found, it will APPEND the field to the end of the list.

In all cases, if any of the new fields don't have an explicit id it will auto generate a default one for them (the id value doesn't really matter and it is mostly used as a stable identifier in case of a field rename).

**Parameters**

- `Rest ...fields: core.Field[]`

**Returns void**

### addAt

```ts
addAt(pos, ...fields): void
```

AddAt is the same as Add but insert/move the fields at the specific position.

If pos < 0, then this method acts the same as calling Add.

If pos > FieldsList total items, then the specified fields are inserted/moved at the end of the list.

**Parameters**

- `pos: number`
- `Rest ...fields: core.Field[]`

**Returns void**

### addMarshaledJSON

```ts
addMarshaledJSON(rawJSON): void
```

AddMarshaledJSON parses the provided raw json data and adds the found fields into the current list (following the same rule as the Add method).

The rawJSON argument could be one of:

```
  - serialized array of field objects
  - single field object.
```

Example:

```
 l.AddMarshaledJSON([]byte{`{"type":"text", name: "test"}`})
 l.AddMarshaledJSON([]byte{`[{"type":"text", name: "test1"}, {"type":"text", name: "test2"}]`})
```

**Parameters**

- `rawJSON: string | number[]`

**Returns void**

### addMarshaledJSONAt

```ts
addMarshaledJSONAt(pos, rawJSON): void
```

AddMarshaledJSONAt is the same as AddMarshaledJSON but insert/move the fields at the specific position.

If pos < 0, then this method acts the same as calling AddMarshaledJSON.

If pos > FieldsList total items, then the specified fields are inserted/moved at the end of the list.

**Parameters**

- `pos: number`
- `rawJSON: string | number[]`

**Returns void**

### asMap

```ts
asMap(): _TygojaDict
```

AsMap returns a map with all registered list field. The returned map is indexed with each field name.

**Returns _TygojaDict**

### clone

```ts
clone(): core.FieldsList
```

Clone creates a deep clone of the current list.

**Returns core.FieldsList**

### fieldNames

```ts
fieldNames(): string[]
```

FieldNames returns a slice with the name of all list fields.

**Returns string[]**

### getById

```ts
getById(fieldId): core.Field
```

GetById returns a single field by its id.

**Parameters**

- `fieldId: string`

**Returns core.Field**

### getByName

```ts
getByName(fieldName): core.Field
```

GetByName returns a single field by its name.

**Parameters**

- `fieldName: string`

**Returns core.Field**

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements the [json.Marshaler] interface.

**Returns string | number[]**

### removeById

```ts
removeById(fieldId): void
```

RemoveById removes a single field by its id.

This method does nothing if field with the specified id doesn't exist.

**Parameters**

- `fieldId: string`

**Returns void**

### removeByName

```ts
removeByName(fieldName): void
```

RemoveByName removes a single field by its name.

This method does nothing if field with the specified name doesn't exist.

**Parameters**

- `fieldName: string`

**Returns void**

### scan

```ts
scan(value): void
```

Scan implements [sql.Scanner] interface to scan the provided value into the current FieldsList instance.

**Parameters**

- `value: any`

**Returns void**

### string

```ts
string(): string
```

String returns the string representation of the current list.

**Returns string**

### unmarshalJSON

```ts
unmarshalJSON(data): void
```

UnmarshalJSON implements [json.Unmarshaler] and loads the provided json data into the current FieldsList.

**Parameters**

- `data: string | number[]`

**Returns void**

### value

```ts
value(): any
```

Value implements the [driver.Valuer] interface.

**Returns any**
