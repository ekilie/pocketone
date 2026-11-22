Source: https://pocketbase.io/jsvm/interfaces/cron.Job.html

# Interface Job

Job defines a single registered cron job.

**Hierarchy**

- Job

## Methods

### expression

```ts
expression(): string
```

Expression returns the plain cron job schedule expression.

**Returns string**

### id

```ts
id(): string
```

Id returns the cron job id.

**Returns string**

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements [json.Marshaler] and export the current jobs data into valid JSON.

**Returns string | number[]**

### run

```ts
run(): void
```

Run runs the cron job function.

**Returns void**
