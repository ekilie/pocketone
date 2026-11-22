Source: https://pocketbase.io/jsvm/functions/_app.txInfo.html

# Function txInfo

```ts
txInfo(): TxAppInfo
```

TxInfo returns the transaction associated with the current app instance (if any).

Could be used if you want to execute indirectly a function after the related app transaction completes using `app.TxInfo().OnAfterFunc(callback)`.

**Returns TxAppInfo**
