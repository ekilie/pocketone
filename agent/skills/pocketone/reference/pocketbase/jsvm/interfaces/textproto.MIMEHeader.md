Source: https://pocketbase.io/jsvm/interfaces/textproto.MIMEHeader.html

# Interface MIMEHeader

A MIMEHeader represents a MIME-style header mapping keys to sets of values.

**Hierarchy**

- [_TygojaDict](https://pocketbase.io/jsvm/types/_TygojaDict.html)

  - MIMEHeader

## Methods

### add

```ts
add(key, value): void
```

Add adds the key, value pair to the header. It appends to any existing values associated with key.

**Parameters**

- `key: string`
- `value: string`

**Returns void**

### del

```ts
del(key): void
```

Del deletes the values associated with key.

**Parameters**

- `key: string`

**Returns void**

### get

```ts
get(key): string
```

Get gets the first value associated with the given key. It is case insensitive; [CanonicalMIMEHeaderKey] is used to canonicalize the provided key. If there are no values associated with the key, Get returns "". To use non-canonical keys, access the map directly.

**Parameters**

- `key: string`

**Returns string**

### set

```ts
set(key, value): void
```

Set sets the header entries associated with key to the single element value. It replaces any existing values associated with key.

**Parameters**

- `key: string`
- `value: string`

**Returns void**

### values

```ts
values(key): string[]
```

Values returns all values associated with the given key. It is case insensitive; [CanonicalMIMEHeaderKey] is used to canonicalize the provided key. To use non-canonical keys, access the map directly. The returned slice is not a copy.

**Parameters**

- `key: string`

**Returns string[]**
