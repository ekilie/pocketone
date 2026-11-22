Source: https://pocketbase.io/jsvm/interfaces/url.Values.html

# Interface Values

Values maps a string key to a list of values. It is typically used for query parameters and form values. Unlike in the http.Header map, the keys in a Values map are case-sensitive.

**Hierarchy**

- [_TygojaDict](https://pocketbase.io/jsvm/types/_TygojaDict.html)

  - Values

## Methods

### add

```ts
add(key, value): void
```

Add adds the value to key. It appends to any existing values associated with key.

**Parameters**

- `key: string`
- `value: string`

**Returns void**

### clone

```ts
clone(): Values
```

Clone creates a deep copy of the subject [Values].

**Returns Values**

### del

```ts
del(key): void
```

Del deletes the values associated with key.

**Parameters**

- `key: string`

**Returns void**

### encode

```ts
encode(): string
```

Encode encodes the values into “URL encoded” form ("bar=baz&foo=quux") sorted by key.

**Returns string**

### get

```ts
get(key): string
```

Get gets the first value associated with the given key. If there are no values associated with the key, Get returns the empty string. To access multiple values, use the map directly.

**Parameters**

- `key: string`

**Returns string**

### has

```ts
has(key): boolean
```

Has checks whether a given key is set.

**Parameters**

- `key: string`

**Returns boolean**

### set

```ts
set(key, value): void
```

Set sets the key to value. It replaces any existing values.

**Parameters**

- `key: string`
- `value: string`

**Returns void**
