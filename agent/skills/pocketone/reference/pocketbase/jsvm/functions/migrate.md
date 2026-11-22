Source: https://pocketbase.io/jsvm/functions/migrate.html

# Function migrate

```ts
migrate(up, down?): void
```

Migrate defines a single migration upgrade/downgrade action.

*Note that this method is available only in pb_migrations context.*

**Parameters**

- `up: ((txApp) => void)`
- `Optional down: ((txApp) => void)`

**Returns void**
