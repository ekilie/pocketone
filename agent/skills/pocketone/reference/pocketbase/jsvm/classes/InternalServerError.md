Source: https://pocketbase.io/jsvm/classes/InternalServerError.html

# Class InternalServerError

InternalServerError returns 429 ApiError.

**Hierarchy**

- [router](https://pocketbase.io/jsvm/modules/router.html).[ApiError](https://pocketbase.io/jsvm/interfaces/router.ApiError.html)

  - InternalServerError

**Implements**

- [router](https://pocketbase.io/jsvm/modules/router.html).[ApiError](https://pocketbase.io/jsvm/interfaces/router.ApiError.html)

## Constructors

### constructor

```ts
new InternalServerError(message?, data?): InternalServerError
```

**Parameters**

- `Optional message: string`
- `Optional data: any`

**Returns InternalServerError**

## Methods

### error

```ts
error(): string
```

Error makes it compatible with the `error` interface.

**Returns string**

### is

```ts
is(target): boolean
```

Is reports whether the current ApiError wraps the target.

**Parameters**

- `target: Error`

**Returns boolean**

### rawData

```ts
rawData(): any
```

RawData returns the unformatted error data (could be an internal error, text, etc.)

**Returns any**

## Properties

### data

```ts
data: _TygojaDict
```

### message

```ts
message: string
```

### status

```ts
status: number
```
