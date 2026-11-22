Source: https://pocketbase.io/jsvm/classes/DynamicModel.html

# Class DynamicModel

DynamicModel creates a new dynamic model with fields from the provided data shape.

Caveats:

- In order to use 0 as double/float initialization number you have to negate it (`-0`).
- You need to use lowerCamelCase when accessing the model fields (e.g. `model.roles` and not `model.Roles` even if in the model shape and in the DB table the column is capitalized).
- Objects are loaded into types.JSONMap, meaning that they need to be accessed with `get(key)` (e.g. `model.meta.get('something')`).
- For describing nullable types you can use the `null*()` helpers - `nullString()`, `nullInt()`, `nullFloat()`, `nullBool()`, `nullArray()`, `nullObject()`.

Example:

```js
const model = new DynamicModel({
    name:       ""     // or nullString() if nullable
    age:        0,     // or nullInt() if nullable
    totalSpent: -0,    // or nullFloat() if nullable
    active:     false, // or nullBool() if nullable
    Roles:      [],    // or nullArray() if nullable; maps to "Roles" in the DB/JSON but the prop would be accessible via "model.roles"
    meta:       {},    // or nullObject() if nullable
})
```

**Hierarchy**

- DynamicModel

**Indexable**

```ts
[key: string]: any
```

## Constructors

### constructor

```ts
new DynamicModel(shape?): DynamicModel
```

**Parameters**

- `Optional shape: {     [key: string]: any; }`

**Returns DynamicModel**
