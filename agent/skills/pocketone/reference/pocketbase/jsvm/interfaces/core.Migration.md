Source: https://pocketbase.io/jsvm/interfaces/core.Migration.html

# Interface Migration

**Hierarchy**

- Migration

## Properties

### down

```ts
down: ((txApp) => void)
```

**Type declaration**

```ts
(txApp): void
```

**Parameters**

- `txApp: App`

**Returns void**

### file

```ts
file: string
```

### reapplyCondition

```ts
reapplyCondition: ((txApp, runner, fileName) => boolean)
```

**Type declaration**

```ts
(txApp, runner, fileName): boolean
```

**Parameters**

- `txApp: App`
- `runner: MigrationsRunner`
- `fileName: string`

**Returns boolean**

### up

```ts
up: ((txApp) => void)
```

**Type declaration**

```ts
(txApp): void
```

**Parameters**

- `txApp: App`

**Returns void**
