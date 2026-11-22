Source: https://pocketbase.io/jsvm/interfaces/types.DateTime.html

# Interface DateTime

DateTime represents a [time.Time] instance in UTC that is wrapped and serialized using the app default date layout.

**Hierarchy**

- DateTime

  - [DateTime](https://pocketbase.io/jsvm/classes/DateTime.html)

**Implemented by**

- [DateTime](https://pocketbase.io/jsvm/classes/DateTime.html)

## Methods

### add

```ts
add(duration): types.DateTime
```

Add returns a new DateTime based on the current DateTime + the specified duration.

**Parameters**

- `duration: Duration`

**Returns types.DateTime**

### addDate

```ts
addDate(years, months, days): types.DateTime
```

AddDate returns a new DateTime based on the current one + duration.

It follows the same rules as [time.AddDate].

**Parameters**

- `years: number`
- `months: number`
- `days: number`

**Returns types.DateTime**

### after

```ts
after(u): boolean
```

After reports whether the current DateTime instance is after u.

**Parameters**

- `u: types.DateTime`

**Returns boolean**

### before

```ts
before(u): boolean
```

Before reports whether the current DateTime instance is before u.

**Parameters**

- `u: types.DateTime`

**Returns boolean**

### compare

```ts
compare(u): number
```

Compare compares the current DateTime instance with u. If the current instance is before u, it returns -1. If the current instance is after u, it returns +1. If they're the same, it returns 0.

**Parameters**

- `u: types.DateTime`

**Returns number**

### equal

```ts
equal(u): boolean
```

Equal reports whether the current DateTime and u represent the same time instant. Two DateTime can be equal even if they are in different locations. For example, 6:00 +0200 and 4:00 UTC are Equal.

**Parameters**

- `u: types.DateTime`

**Returns boolean**

### isZero

```ts
isZero(): boolean
```

IsZero checks whether the current DateTime instance has zero time value.

**Returns boolean**

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements the [json.Marshaler] interface.

**Returns string | number[]**

### scan

```ts
scan(value): void
```

Scan implements [sql.Scanner] interface to scan the provided value into the current DateTime instance.

**Parameters**

- `value: any`

**Returns void**

### string

```ts
string(): string
```

String serializes the current DateTime instance into a formatted UTC date string.

The zero value is serialized to an empty string.

**Returns string**

### sub

```ts
sub(u): Duration
```

Sub returns a [time.Duration] by subtracting the specified DateTime from the current one.

If the result exceeds the maximum (or minimum) value that can be stored in a [time.Duration], the maximum (or minimum) duration will be returned.

**Parameters**

- `u: types.DateTime`

**Returns Duration**

### time

```ts
time(): Time
```

Time returns the internal [time.Time] instance.

**Returns Time**

### unix

```ts
unix(): number
```

Unix returns the current DateTime as a Unix time, aka. the number of seconds elapsed since January 1, 1970 UTC.

**Returns number**

### unmarshalJSON

```ts
unmarshalJSON(b): void
```

UnmarshalJSON implements the [json.Unmarshaler] interface.

**Parameters**

- `b: string | number[]`

**Returns void**

### value

```ts
value(): any
```

Value implements the [driver.Valuer] interface.

**Returns any**
