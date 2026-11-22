Source: https://pocketbase.io/jsvm/functions/cronRemove.html

# Function cronRemove

```ts
cronRemove(jobId): void
```

CronRemove removes a single registered cron job by its name.

Example:

```js
cronRemove("hello")
```

*Note that this method is available only in pb_hooks context.*

**Parameters**

- `jobId: string`

**Returns void**
