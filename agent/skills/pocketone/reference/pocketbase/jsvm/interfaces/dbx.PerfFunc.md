Source: https://pocketbase.io/jsvm/interfaces/dbx.PerfFunc.html

# Interface PerfFunc

PerfFunc is called when a query finishes execution. The query execution time is passed to this function so that the DB performance can be profiled. The "ns" parameter gives the number of nanoseconds that the SQL statement takes to execute, while the "execute" parameter indicates whether the SQL statement is executed or queried (usually SELECT statements).

**Hierarchy**

- PerfFunc

```ts
PerfFunc(ns, sql, execute): void
```

**Parameters**

- `ns: number`
- `sql: string`
- `execute: boolean`

**Returns void**
