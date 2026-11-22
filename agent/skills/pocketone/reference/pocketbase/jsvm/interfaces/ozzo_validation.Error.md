Source: https://pocketbase.io/jsvm/interfaces/ozzo_validation.Error.html

# Interface Error

Error interface represents an validation error

**Hierarchy**

- Error

  - [ValidationError](https://pocketbase.io/jsvm/classes/ValidationError.html)

**Implemented by**

- [ValidationError](https://pocketbase.io/jsvm/classes/ValidationError.html)

**Indexable**

```ts
[key: string]: any
```

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
