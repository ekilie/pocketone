Source: https://pocketbase.io/jsvm/interfaces/sql.NullString.html

# Interface NullString

NullString represents a string that may be null. NullString implements the [Scanner] interface so it can be used as a scan destination:

```
  var s NullString
  err := db.QueryRow("SELECT name FROM foo WHERE id=?", id).Scan(&s)
  ...
  if s.Valid {
     // use s.String
  } else {
     // NULL value
  }
```

**Hierarchy**

- NullString

## Methods

### scan

```ts
scan(value): void
```

Scan implements the [Scanner] interface.

**Parameters**

- `value: any`

**Returns void**

### value

```ts
value(): any
```

Value implements the [driver.Valuer] interface.

**Returns any**

## Properties

### string

```ts
string: string
```

### valid

```ts
valid: boolean
```
