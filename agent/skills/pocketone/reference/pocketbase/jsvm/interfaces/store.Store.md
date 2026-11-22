Source: https://pocketbase.io/jsvm/interfaces/store.Store.html

# Interface Store<K, T>

Store defines a concurrent safe in memory key-value data store.

**Type Parameters**

- K
- T

**Hierarchy**

- Store

## Methods

### get

```ts
get(key): T
```

Get returns a single element value from the store.

If key is not set, the zero T value is returned.

**Parameters**

- `key: K`

**Returns T**

### getAll

```ts
getAll(): _TygojaDict
```

GetAll returns a shallow copy of the current store data.

**Returns _TygojaDict**

### getOk

```ts
getOk(key): [T, boolean]
```

GetOk is similar to Get but returns also a boolean indicating whether the key exists or not.

**Parameters**

- `key: K`

**Returns [T, boolean]**

### getOrSet

```ts
getOrSet(key, setFunc): T
```

GetOrSet retrieves a single existing value for the provided key or stores a new one if it doesn't exist.

**Parameters**

- `key: K`
- `setFunc: (() => T)`

**Returns T**

### has

```ts
has(key): boolean
```

Has checks if element with the specified key exist or not.

**Parameters**

- `key: K`

**Returns boolean**

### keys

```ts
keys(): K[]
```

Keys returns a slice with all of the store keys.

**Returns K[]**

### length

```ts
length(): number
```

Length returns the current number of elements in the store.

**Returns number**

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements [json.Marshaler] and export the current store data into valid JSON.

**Returns string | number[]**

### remove

```ts
remove(key): void
```

Remove removes a single entry from the store.

Remove does nothing if key doesn't exist in the store.

**Parameters**

- `key: K`

**Returns void**

### removeAll

```ts
removeAll(): void
```

RemoveAll removes all the existing store entries.

**Returns void**

### reset

```ts
reset(newData): void
```

Reset clears the store and replaces the store data with a shallow copy of the provided newData.

**Parameters**

- `newData: _TygojaDict`

**Returns void**

### set

```ts
set(key, value): void
```

Set sets (or overwrite if already exists) a new value for key.

**Parameters**

- `key: K`
- `value: T`

**Returns void**

### setFunc

```ts
setFunc(key, fn): void
```

SetFunc sets (or overwrite if already exists) a new value resolved from the function callback for the provided key.

The function callback receives as argument the old store element value (if exists). If there is no old store element, the argument will be the T zero value.

Example:

```
 s := store.New[string, int](nil)
 s.SetFunc("count", func(old int) int {
     return old + 1
 })
```

**Parameters**

- `key: K`
- `fn: ((old) => T)`

**Returns void**

### setIfLessThanLimit

```ts
setIfLessThanLimit(key, value, maxAllowedElements): boolean
```

SetIfLessThanLimit sets (or overwrite if already exist) a new value for key.

This method is similar to Set() but **it will skip adding new elements** to the store if the store length has reached the specified limit. false is returned if maxAllowedElements limit is reached.

**Parameters**

- `key: K`
- `value: T`
- `maxAllowedElements: number`

**Returns boolean**

### unmarshalJSON

```ts
unmarshalJSON(data): void
```

UnmarshalJSON implements [json.Unmarshaler] and imports the provided JSON data into the store.

The store entries that match with the ones from the data will be overwritten with the new value.

**Parameters**

- `data: string | number[]`

**Returns void**

### values

```ts
values(): T[]
```

Values returns a slice with all of the store values.

**Returns T[]**
