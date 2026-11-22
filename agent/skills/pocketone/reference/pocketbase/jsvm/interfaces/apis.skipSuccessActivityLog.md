Source: https://pocketbase.io/jsvm/interfaces/apis.skipSuccessActivityLog.html

# Interface skipSuccessActivityLog

**Hierarchy**

- skipSuccessActivityLog

```ts
skipSuccessActivityLog(): hook.Handler<RequestEvent>
```

SkipSuccessActivityLog is a helper middleware that instructs the global activity logger to log only requests that have failed/returned an error.

**Returns hook.Handler<RequestEvent>**
