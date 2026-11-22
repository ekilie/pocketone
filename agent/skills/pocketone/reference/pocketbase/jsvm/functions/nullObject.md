Source: https://pocketbase.io/jsvm/functions/nullObject.html

# Function nullObject

```ts
nullObject(): {     get(key) => any;     set(key, value) => void; }
```

nullObject creates an empty Go types.JSONMap pointer usually used for describing a **nullable** `DynamicModel` JSON object value.

**Returns {     get(key) => any;     set(key, value) => void; }**

**get:function**

```ts
get(key): any
```

**Parameters**

- `key: string`

**Returns any**

**set:function**

```ts
set(key, value): void
```

**Parameters**

- `key: string`
- `value: any`

**Returns void**
