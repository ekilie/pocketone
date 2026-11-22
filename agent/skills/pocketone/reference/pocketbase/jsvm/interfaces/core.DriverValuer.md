Source: https://pocketbase.io/jsvm/interfaces/core.DriverValuer.html

# Interface DriverValuer

DriverValuer defines a Field interface for exporting and formatting a field value for the database.

**Hierarchy**

- DriverValuer

**Indexable**

```ts
[key: string]: any
```

## Methods

### driverValue

```ts
driverValue(record): any
```

DriverValue exports a single field value for persistence in the database.

**Parameters**

- `record: core.Record`

**Returns any**
