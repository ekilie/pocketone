Source: https://pocketbase.io/jsvm/interfaces/types.JSONRaw.html

# Interface JSONRaw

JSONRaw defines a json value type that is safe for db read/write.

**Hierarchy**

- Array<number>

  - JSONRaw

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

Scan implements [sql.Scanner] interface to scan the provided value into the current JSONRaw instance.

**Parameters**

- `value: any`

**Returns void**

### string

```ts
string(): string
```

String returns the current JSONRaw instance as a json encoded string.

**Returns string**

### unmarshalJSON

```ts
unmarshalJSON(b): void
```

UnmarshalJSON implements the [json.Unmarshaler] interface.

**Parameters**

- `b: string | number[]`

**Returns void**

### value

```ts
value(): any
```

Value implements the [driver.Valuer] interface.

**Returns any**
