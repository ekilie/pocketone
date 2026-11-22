Source: https://pocketbase.io/jsvm/interfaces/os.isTimeout.html

# Interface isTimeout

**Hierarchy**

- isTimeout

```ts
isTimeout(err): boolean
```

IsTimeout returns a boolean indicating whether its argument is known to report that a timeout occurred.

This function predates [errors.Is], and the notion of whether an error indicates a timeout can be ambiguous. For example, the Unix error EWOULDBLOCK sometimes indicates a timeout and sometimes does not. New code should use errors.Is with a value appropriate to the call returning the error, such as [os.ErrDeadlineExceeded].

**Parameters**

- `err: Error`

**Returns boolean**
