Source: https://pocketbase.io/jsvm/interfaces/core.TxAppInfo.html

# Interface TxAppInfo

TxAppInfo represents an active transaction context associated to an existing app instance.

**Hierarchy**

- TxAppInfo

## Methods

### onComplete

```ts
onComplete(fn): void
```

OnComplete registers the provided callback that will be invoked once the related transaction ends (either completes successfully or rollbacked with an error).

The callback receives the transaction error (if any) as its argument. Any additional errors returned by the OnComplete callbacks will be joined together with txErr when returning the final transaction result.

**Parameters**

- `fn: ((txErr) => void)`

**Returns void**
