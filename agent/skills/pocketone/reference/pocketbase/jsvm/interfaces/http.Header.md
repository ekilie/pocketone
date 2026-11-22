Source: https://pocketbase.io/jsvm/interfaces/http.Header.html

# Interface Header

A Header represents the key-value pairs in an HTTP header.

The keys should be in canonical form, as returned by [CanonicalHeaderKey].

**Hierarchy**

- [_TygojaDict](https://pocketbase.io/jsvm/types/_TygojaDict.html)

  - Header

## Methods

### add

```ts
add(key, value): void
```

Add adds the key, value pair to the header. It appends to any existing values associated with key. The key is case insensitive; it is canonicalized by [CanonicalHeaderKey].

**Parameters**

- `key: string`
- `value: string`

**Returns void**

### clone

```ts
clone(): Header
```

Clone returns a copy of h or nil if h is nil.

**Returns Header**

### del

```ts
del(key): void
```

Del deletes the values associated with key. The key is case insensitive; it is canonicalized by [CanonicalHeaderKey].

**Parameters**

- `key: string`

**Returns void**

### get

```ts
get(key): string
```

Get gets the first value associated with the given key. If there are no values associated with the key, Get returns "". It is case insensitive; [textproto.CanonicalMIMEHeaderKey] is used to canonicalize the provided key. Get assumes that all keys are stored in canonical form. To use non-canonical keys, access the map directly.

**Parameters**

- `key: string`

**Returns string**

### set

```ts
set(key, value): void
```

Set sets the header entries associated with key to the single element value. It replaces any existing values associated with key. The key is case insensitive; it is canonicalized by [textproto.CanonicalMIMEHeaderKey]. To use non-canonical keys, assign to the map directly.

**Parameters**

- `key: string`
- `value: string`

**Returns void**

### values

```ts
values(key): string[]
```

Values returns all values associated with the given key. It is case insensitive; [textproto.CanonicalMIMEHeaderKey] is used to canonicalize the provided key. To use non-canonical keys, access the map directly. The returned slice is not a copy.

**Parameters**

- `key: string`

**Returns string[]**

### write

```ts
write(w): void
```

Write writes a header in wire format.

**Parameters**

- `w: io.Writer`

**Returns void**

### writeSubset

```ts
writeSubset(w, exclude): void
```

WriteSubset writes a header in wire format. If exclude is not nil, keys where exclude[key] == true are not written. Keys are not canonicalized before checking the exclude map.

**Parameters**

- `w: io.Writer`
- `exclude: _TygojaDict`

**Returns void**
