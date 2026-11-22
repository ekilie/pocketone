Source: https://pocketbase.io/jsvm/functions/toBytes.html

# Function toBytes

```ts
toBytes(val, maxBytes?): number[]
```

toBytes converts the specified value into a bytes slice.

Support optional second maxBytes argument to limit the max read bytes when the value is a io.Reader (default to 32MB).

Types that don't have Go slice representation (bool, objects, etc.) are serialized to UTF8 string and its bytes slice is returned.

Example:

```js
// io.Reader
const ex1 = toBytes(e.request.body)

// string
const ex2 = toBytes("hello") // [104 101 108 108 111]

// object (the same as the string '{"test":1}')
const ex3 = toBytes({"test":1}) // [123 34 116 101 115 116 34 58 49 125]

// null
const ex4 = toBytes(null) // []
```

**Parameters**

- `val: any`
- `Optional maxBytes: number`

**Returns number[]**
