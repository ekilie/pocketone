Source: https://pocketbase.io/jsvm/functions/cronAdd.html

# Function cronAdd

```ts
cronAdd(jobId, cronExpr, handler): void
```

CronAdd registers a new cron job.

If a cron job with the specified name already exist, it will be replaced with the new one.

Example:

```js
// prints "Hello world!" on every 30 minutes
cronAdd("hello", "*/30 * * * *", () => {
    console.log("Hello world!")
})
```

*Note that this method is available only in pb_hooks context.*

**Parameters**

- `jobId: string`
- `cronExpr: string`
- `handler: (() => void)`

**Returns void**
