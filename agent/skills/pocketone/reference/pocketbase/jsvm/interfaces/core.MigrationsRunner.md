Source: https://pocketbase.io/jsvm/interfaces/core.MigrationsRunner.html

# Interface MigrationsRunner

MigrationsRunner defines a simple struct for managing the execution of db migrations.

**Hierarchy**

- MigrationsRunner

## Methods

### down

```ts
down(toRevertCount): string[]
```

Down reverts the last `toRevertCount` applied migrations (in the order they were applied).

On success returns list with the reverted migrations file names.

**Parameters**

- `toRevertCount: number`

**Returns string[]**

### removeMissingAppliedMigrations

```ts
removeMissingAppliedMigrations(): void
```

RemoveMissingAppliedMigrations removes the db entries of all applied migrations that are not listed in the runner's migrations list.

**Returns void**

### run

```ts
run(...args): void
```

Run interactively executes the current runner with the provided args.

The following commands are supported:

- up - applies all migrations
- down [n] - reverts the last n (default 1) applied migrations
- history-sync - syncs the migrations table with the runner's migrations list

**Parameters**

- `Rest ...args: string[]`

**Returns void**

### up

```ts
up(): string[]
```

Up executes all unapplied migrations for the provided runner.

On success returns list with the applied migrations file names.

**Returns string[]**
