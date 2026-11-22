Source: https://pocketbase.io/jsvm/interfaces/sql.ColumnType.html

# Interface ColumnType

ColumnType contains the name and type of a column.

**Hierarchy**

- ColumnType

## Methods

### databaseTypeName

```ts
databaseTypeName(): string
```

DatabaseTypeName returns the database system name of the column type. If an empty string is returned, then the driver type name is not supported. Consult your driver documentation for a list of driver data types. [ColumnType.Length] specifiers are not included. Common type names include "VARCHAR", "TEXT", "NVARCHAR", "DECIMAL", "BOOL", "INT", and "BIGINT".

**Returns string**

### decimalSize

```ts
decimalSize(): [number, number, boolean]
```

DecimalSize returns the scale and precision of a decimal type. If not applicable or if not supported ok is false.

**Returns [number, number, boolean]**

### length

```ts
length(): [number, boolean]
```

Length returns the column type length for variable length column types such as text and binary field types. If the type length is unbounded the value will be [math.MaxInt64] (any database limits will still apply). If the column type is not variable length, such as an int, or if not supported by the driver ok is false.

**Returns [number, boolean]**

### name

```ts
name(): string
```

Name returns the name or alias of the column.

**Returns string**

### nullable

```ts
nullable(): [boolean, boolean]
```

Nullable reports whether the column may be null. If a driver does not support this property ok will be false.

**Returns [boolean, boolean]**

### scanType

```ts
scanType(): any
```

ScanType returns a Go type suitable for scanning into using [Rows.Scan]. If a driver does not support this property ScanType will return the type of an empty interface.

**Returns any**
