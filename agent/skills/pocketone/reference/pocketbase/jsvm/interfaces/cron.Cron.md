Source: https://pocketbase.io/jsvm/interfaces/cron.Cron.html

# Interface Cron

Cron is a crontab-like struct for tasks/jobs scheduling.

**Hierarchy**

- Cron

## Methods

### add

```ts
add(jobId, cronExpr, fn): void
```

Add registers a single cron job.

If there is already a job with the provided id, then the old job will be replaced with the new one.

cronExpr is a regular cron expression, eg. "0 */3 * * *" (aka. at minute 0 past every 3rd hour). Check cron.NewSchedule() for the supported tokens.

**Parameters**

- `jobId: string`
- `cronExpr: string`
- `fn: (() => void)`

**Returns void**

### hasStarted

```ts
hasStarted(): boolean
```

HasStarted checks whether the current Cron ticker has been started.

**Returns boolean**

### jobs

```ts
jobs(): Job[]
```

Jobs returns a shallow copy of the currently registered cron jobs.

**Returns Job[]**

### mustAdd

```ts
mustAdd(jobId, cronExpr, run): void
```

MustAdd is similar to Add() but panic on failure.

**Parameters**

- `jobId: string`
- `cronExpr: string`
- `run: (() => void)`

**Returns void**

### remove

```ts
remove(jobId): void
```

Remove removes a single cron job by its id.

**Parameters**

- `jobId: string`

**Returns void**

### removeAll

```ts
removeAll(): void
```

RemoveAll removes all registered cron jobs.

**Returns void**

### setInterval

```ts
setInterval(d): void
```

SetInterval changes the current cron tick interval (it usually should be >= 1 minute).

**Parameters**

- `d: Duration`

**Returns void**

### setTimezone

```ts
setTimezone(l): void
```

SetTimezone changes the current cron tick timezone.

**Parameters**

- `l: Location`

**Returns void**

### start

```ts
start(): void
```

Start starts the cron ticker.

Calling Start() on already started cron will restart the ticker.

**Returns void**

### stop

```ts
stop(): void
```

Stop stops the current cron ticker (if not already).

You can resume the ticker by calling Start().

**Returns void**

### total

```ts
total(): number
```

Total returns the current total number of registered cron jobs.

**Returns number**
