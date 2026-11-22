Source: https://pocketbase.io/jsvm/interfaces/time.Duration.html

# Interface Duration

A Duration represents the elapsed time between two instants as an int64 nanosecond count. The representation limits the largest representable duration to approximately 290 years.

**Hierarchy**

- Number

  - Duration

## Methods

### abs

```ts
abs(): Duration
```

Abs returns the absolute value of d. As a special case, Duration([math.MinInt64]) is converted to Duration([math.MaxInt64]), reducing its magnitude by 1 nanosecond.

**Returns Duration**

### hours

```ts
hours(): number
```

Hours returns the duration as a floating point number of hours.

**Returns number**

### microseconds

```ts
microseconds(): number
```

Microseconds returns the duration as an integer microsecond count.

**Returns number**

### milliseconds

```ts
milliseconds(): number
```

Milliseconds returns the duration as an integer millisecond count.

**Returns number**

### minutes

```ts
minutes(): number
```

Minutes returns the duration as a floating point number of minutes.

**Returns number**

### nanoseconds

```ts
nanoseconds(): number
```

Nanoseconds returns the duration as an integer nanosecond count.

**Returns number**

### round

```ts
round(m): Duration
```

Round returns the result of rounding d to the nearest multiple of m. The rounding behavior for halfway values is to round away from zero. If the result exceeds the maximum (or minimum) value that can be stored in a [Duration], Round returns the maximum (or minimum) duration. If m <= 0, Round returns d unchanged.

**Parameters**

- `m: Duration`

**Returns Duration**

### seconds

```ts
seconds(): number
```

Seconds returns the duration as a floating point number of seconds.

**Returns number**

### string

```ts
string(): string
```

String returns a string representing the duration in the form "72h3m0.5s". Leading zero units are omitted. As a special case, durations less than one second format use a smaller unit (milli-, micro-, or nanoseconds) to ensure that the leading digit is non-zero. The zero duration formats as 0s.

**Returns string**

### truncate

```ts
truncate(m): Duration
```

Truncate returns the result of rounding d toward zero to a multiple of m. If m <= 0, Truncate returns d unchanged.

**Parameters**

- `m: Duration`

**Returns Duration**
