Source: https://pocketbase.io/jsvm/functions/_apis.skipSuccessActivityLog.html

# Function skipSuccessActivityLog

```ts
skipSuccessActivityLog(): hook.Handler<RequestEvent>
```

SkipSuccessActivityLog is a helper middleware that instructs the global activity logger to log only requests that have failed/returned an error.

**Returns hook.Handler<RequestEvent>**
