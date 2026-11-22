Source: https://pocketbase.io/jsvm/interfaces/types.JSONMap.html

# Interface JSONMap<T>

JSONMap defines a map that is safe for json and db read/write.

**Type Parameters**

- T

**Hierarchy**

- [_TygojaDict](https://pocketbase.io/jsvm/types/_TygojaDict.html)

  - JSONMap

## Methods

### get

```ts
get(key): T
```

Get retrieves a single value from the current JSONMap[T].

This helper was added primarily to assist the goja integration since custom map types don't have direct access to the map keys ([https://pkg.go.dev/github.com/dop251/goja#hdr-Maps_with_methods](https://pkg.go.dev/github.com/dop251/goja#hdr-Maps_with_methods)).

**Parameters**

- `key: string`

**Returns T**

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

Scan implements [sql.Scanner] interface to scan the provided value into the current JSONMap[T] instance.

**Parameters**

- `value: any`

**Returns void**

### set

```ts
set(key, value): void
```

Set sets a single value in the current JSONMap[T].

This helper was added primarily to assist the goja integration since custom map types don't have direct access to the map keys ([https://pkg.go.dev/github.com/dop251/goja#hdr-Maps_with_methods](https://pkg.go.dev/github.com/dop251/goja#hdr-Maps_with_methods)).

**Parameters**

- `key: string`
- `value: T`

**Returns void**

### string

```ts
string(): string
```

String returns the string representation of the current json map.

**Returns string**

### value

```ts
value(): any
```

Value implements the [driver.Valuer] interface.

**Returns any**
