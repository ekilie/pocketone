Source: https://pocketbase.io/jsvm/classes/ValidationError.html

# Class ValidationError

ValidationError defines a single formatted data validation error, usually used as part of an error response.

```js
new ValidationError("invalid_title", "Title is not valid")
```

**Hierarchy**

- [Error](https://pocketbase.io/jsvm/interfaces/ozzo_validation.Error.html)

  - ValidationError

**Implements**

- [Error](https://pocketbase.io/jsvm/interfaces/ozzo_validation.Error.html)

## Constructors

### constructor

```ts
new ValidationError(code?, message?): ValidationError
```

**Parameters**

- `Optional code: string`
- `Optional message: string`

**Returns ValidationError**

## Methods

### code

```ts
code(): string
```

**Returns string**

### error

```ts
error(): string
```

**Returns string**

### message

```ts
message(): string
```

**Returns string**

### params

```ts
params(): _TygojaDict
```

**Returns _TygojaDict**

### setMessage

```ts
setMessage(_arg0): Error
```

**Parameters**

- `_arg0: string`

**Returns Error**

### setParams

```ts
setParams(_arg0): Error
```

**Parameters**

- `_arg0: _TygojaDict`

**Returns Error**
