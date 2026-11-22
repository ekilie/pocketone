Source: https://pocketbase.io/jsvm/functions/arrayOf.html

# Function arrayOf

```ts
arrayOf<T>(model): T[]
```

arrayOf creates a placeholder array of the specified models. Usually used to populate DB result into an array of models.

Example:

```js
const records = arrayOf(new Record)

$app.recordQuery("articles").limit(10).all(records)
```

**Type Parameters**

- T

**Parameters**

- `model: T`

**Returns T[]**
