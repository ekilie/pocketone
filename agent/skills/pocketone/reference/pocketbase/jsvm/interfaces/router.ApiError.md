Source: https://pocketbase.io/jsvm/interfaces/router.ApiError.html

# Interface ApiError

ApiError defines the struct for a basic api error response.

**Hierarchy**

- ApiError

  - [ApiError](https://pocketbase.io/jsvm/classes/ApiError.html)
  - [NotFoundError](https://pocketbase.io/jsvm/classes/NotFoundError.html)
  - [BadRequestError](https://pocketbase.io/jsvm/classes/BadRequestError.html)
  - [ForbiddenError](https://pocketbase.io/jsvm/classes/ForbiddenError.html)
  - [UnauthorizedError](https://pocketbase.io/jsvm/classes/UnauthorizedError.html)
  - [TooManyRequestsError](https://pocketbase.io/jsvm/classes/TooManyRequestsError.html)
  - [InternalServerError](https://pocketbase.io/jsvm/classes/InternalServerError.html)

**Implemented by**

- [ApiError](https://pocketbase.io/jsvm/classes/ApiError.html)
- [BadRequestError](https://pocketbase.io/jsvm/classes/BadRequestError.html)
- [ForbiddenError](https://pocketbase.io/jsvm/classes/ForbiddenError.html)
- [InternalServerError](https://pocketbase.io/jsvm/classes/InternalServerError.html)
- [NotFoundError](https://pocketbase.io/jsvm/classes/NotFoundError.html)
- [TooManyRequestsError](https://pocketbase.io/jsvm/classes/TooManyRequestsError.html)
- [UnauthorizedError](https://pocketbase.io/jsvm/classes/UnauthorizedError.html)

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
