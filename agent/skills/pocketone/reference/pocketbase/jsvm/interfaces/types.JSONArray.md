Source: https://pocketbase.io/jsvm/interfaces/types.JSONArray.html

# Interface JSONArray<T>

JSONArray defines a slice that is safe for json and db read/write.

**Type Parameters**

- T

**Hierarchy**

- Array<T>

  - JSONArray

## Methods

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements the [json.Marshaler] interface.

**Returns string | number[]**

### scan

```ts
scan(value): void
```

Scan implements [sql.Scanner] interface to scan the provided value into the current JSONArray[T] instance.

**Parameters**

- `value: any`

**Returns void**

### string

```ts
string(): string
```

String returns the string representation of the current json array.

**Returns string**

### value

```ts
value(): any
```

Value implements the [driver.Valuer] interface.

**Returns any**
