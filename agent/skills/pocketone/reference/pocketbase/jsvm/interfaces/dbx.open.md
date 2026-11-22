Source: https://pocketbase.io/jsvm/interfaces/dbx.open.html

# Interface open

**Hierarchy**

- open

```ts
open(driverName, dsn): dbx.DB
```

Open opens a database specified by a driver name and data source name (DSN). Note that Open does not check if DSN is specified correctly. It doesn't try to establish a DB connection either. Please refer to sql.Open() for more information.

**Parameters**

- `driverName: string`
- `dsn: string`

**Returns dbx.DB**
