Source: https://pocketbase.io/jsvm/interfaces/jwt.NumericDate.html

# Interface NumericDate

NumericDate represents a JSON numeric date value, as referenced at [https://datatracker.ietf.org/doc/html/rfc7519#section-2](https://datatracker.ietf.org/doc/html/rfc7519#section-2).

**Hierarchy**

- [_sYAYUqL](https://pocketbase.io/jsvm/types/jwt._sYAYUqL.html)

  - NumericDate

## Methods

### add

```ts
add(d): Time
```

Add returns the time t+d.

**Parameters**

- `d: Duration`

**Returns Time**

### addDate

```ts
addDate(years, months, days): Time
```

AddDate returns the time corresponding to adding the given number of years, months, and days to t. For example, AddDate(-1, 2, 3) applied to January 1, 2011 returns March 4, 2010.

Note that dates are fundamentally coupled to timezones, and calendrical periods like days don't have fixed durations. AddDate uses the Location of the Time value to determine these durations. That means that the same AddDate arguments can produce a different shift in absolute time depending on the base Time value and its Location. For example, AddDate(0, 0, 1) applied to 12:00 on March 27 always returns 12:00 on March 28. At some locations and in some years this is a 24 hour shift. In others it's a 23 hour shift due to daylight savings time transitions.

AddDate normalizes its result in the same way that Date does, so, for example, adding one month to October 31 yields December 1, the normalized form for November 31.

**Parameters**

- `years: number`
- `months: number`
- `days: number`

**Returns Time**

### after

```ts
after(u): boolean
```

After reports whether the time instant t is after u.

**Parameters**

- `u: Time`

**Returns boolean**

### appendBinary

```ts
appendBinary(b): string | number[]
```

AppendBinary implements the [encoding.BinaryAppender] interface.

**Parameters**

- `b: string | number[]`

**Returns string | number[]**

### appendFormat

```ts
appendFormat(b, layout): string | number[]
```

AppendFormat is like [Time.Format] but appends the textual representation to b and returns the extended buffer.

**Parameters**

- `b: string | number[]`
- `layout: string`

**Returns string | number[]**

### appendText

```ts
appendText(b): string | number[]
```

AppendText implements the [encoding.TextAppender] interface. The time is formatted in RFC 3339 format with sub-second precision. If the timestamp cannot be represented as valid RFC 3339 (e.g., the year is out of range), then an error is returned.

**Parameters**

- `b: string | number[]`

**Returns string | number[]**

### before

```ts
before(u): boolean
```

Before reports whether the time instant t is before u.

**Parameters**

- `u: Time`

**Returns boolean**

### clock

```ts
clock(): [number, number, number]
```

Clock returns the hour, minute, and second within the day specified by t.

**Returns [number, number, number]**

### compare

```ts
compare(u): number
```

Compare compares the time instant t with u. If t is before u, it returns -1; if t is after u, it returns +1; if they're the same, it returns 0.

**Parameters**

- `u: Time`

**Returns number**

### date

```ts
date(): [number, Month, number]
```

Date returns the year, month, and day in which t occurs.

**Returns [number, Month, number]**

### day

```ts
day(): number
```

Day returns the day of the month specified by t.

**Returns number**

### equal

```ts
equal(u): boolean
```

Equal reports whether t and u represent the same time instant. Two times can be equal even if they are in different locations. For example, 6:00 +0200 and 4:00 UTC are Equal. See the documentation on the Time type for the pitfalls of using == with Time values; most code should use Equal instead.

**Parameters**

- `u: Time`

**Returns boolean**

### format

```ts
format(layout): string
```

Format returns a textual representation of the time value formatted according to the layout defined by the argument. See the documentation for the constant called [Layout] to see how to represent the layout format.

The executable example for [Time.Format] demonstrates the working of the layout string in detail and is a good reference.

**Parameters**

- `layout: string`

**Returns string**

### goString

```ts
goString(): string
```

GoString implements [fmt.GoStringer] and formats t to be printed in Go source code.

**Returns string**

### gobDecode

```ts
gobDecode(data): void
```

GobDecode implements the gob.GobDecoder interface.

**Parameters**

- `data: string | number[]`

**Returns void**

### gobEncode

```ts
gobEncode(): string | number[]
```

GobEncode implements the gob.GobEncoder interface.

**Returns string | number[]**

### hour

```ts
hour(): number
```

Hour returns the hour within the day specified by t, in the range [0, 23].

**Returns number**

### in

```ts
in(loc): Time
```

In returns a copy of t representing the same time instant, but with the copy's location information set to loc for display purposes.

In panics if loc is nil.

**Parameters**

- `loc: Location`

**Returns Time**

### isDST

```ts
isDST(): boolean
```

IsDST reports whether the time in the configured location is in Daylight Savings Time.

**Returns boolean**

### isZero

```ts
isZero(): boolean
```

IsZero reports whether t represents the zero time instant, January 1, year 1, 00:00:00 UTC.

**Returns boolean**

### isoWeek

```ts
isoWeek(): [number, number]
```

ISOWeek returns the ISO 8601 year and week number in which t occurs. Week ranges from 1 to 53. Jan 01 to Jan 03 of year n might belong to week 52 or 53 of year n-1, and Dec 29 to Dec 31 might belong to week 1 of year n+1.

**Returns [number, number]**

### local

```ts
local(): Time
```

Local returns t with the location set to local time.

**Returns Time**

### location

```ts
location(): Location
```

Location returns the time zone information associated with t.

**Returns Location**

### marshalBinary

```ts
marshalBinary(): string | number[]
```

MarshalBinary implements the [encoding.BinaryMarshaler] interface.

**Returns string | number[]**

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON is an implementation of the json.RawMessage interface and serializes the UNIX epoch represented in NumericDate to a byte array, using the precision specified in TimePrecision.

**Returns string | number[]**

### marshalText

```ts
marshalText(): string | number[]
```

MarshalText implements the [encoding.TextMarshaler] interface. The output matches that of calling the [Time.AppendText] method.

See [Time.AppendText] for more information.

**Returns string | number[]**

### minute

```ts
minute(): number
```

Minute returns the minute offset within the hour specified by t, in the range [0, 59].

**Returns number**

### month

```ts
month(): Month
```

Month returns the month of the year specified by t.

**Returns Month**

### nanosecond

```ts
nanosecond(): number
```

Nanosecond returns the nanosecond offset within the second specified by t, in the range [0, 999999999].

**Returns number**

### round

```ts
round(d): Time
```

Round returns the result of rounding t to the nearest multiple of d (since the zero time). The rounding behavior for halfway values is to round up. If d <= 0, Round returns t stripped of any monotonic clock reading but otherwise unchanged.

Round operates on the time as an absolute duration since the zero time; it does not operate on the presentation form of the time. Thus, Round(Hour) may return a time with a non-zero minute, depending on the time's Location.

**Parameters**

- `d: Duration`

**Returns Time**

### second

```ts
second(): number
```

Second returns the second offset within the minute specified by t, in the range [0, 59].

**Returns number**

### string

```ts
string(): string
```

String returns the time formatted using the format string

```
 "2006-01-02 15:04:05.999999999 -0700 MST"
```

If the time has a monotonic clock reading, the returned string includes a final field "m=±", where value is the monotonic clock reading formatted as a decimal number of seconds.

The returned string is meant for debugging; for a stable serialized representation, use t.MarshalText, t.MarshalBinary, or t.Format with an explicit format string.

**Returns string**

### sub

```ts
sub(u): Duration
```

Sub returns the duration t-u. If the result exceeds the maximum (or minimum) value that can be stored in a [Duration], the maximum (or minimum) duration will be returned. To compute t-d for a duration d, use t.Add(-d).

**Parameters**

- `u: Time`

**Returns Duration**

### truncate

```ts
truncate(d): Time
```

Truncate returns the result of rounding t down to a multiple of d (since the zero time). If d <= 0, Truncate returns t stripped of any monotonic clock reading but otherwise unchanged.

Truncate operates on the time as an absolute duration since the zero time; it does not operate on the presentation form of the time. Thus, Truncate(Hour) may return a time with a non-zero minute, depending on the time's Location.

**Parameters**

- `d: Duration`

**Returns Time**

### unix

```ts
unix(): number
```

Unix returns t as a Unix time, the number of seconds elapsed since January 1, 1970 UTC. The result does not depend on the location associated with t. Unix-like operating systems often record time as a 32-bit count of seconds, but since the method here returns a 64-bit value it is valid for billions of years into the past or future.

**Returns number**

### unixMicro

```ts
unixMicro(): number
```

UnixMicro returns t as a Unix time, the number of microseconds elapsed since January 1, 1970 UTC. The result is undefined if the Unix time in microseconds cannot be represented by an int64 (a date before year -290307 or after year 294246). The result does not depend on the location associated with t.

**Returns number**

### unixMilli

```ts
unixMilli(): number
```

UnixMilli returns t as a Unix time, the number of milliseconds elapsed since January 1, 1970 UTC. The result is undefined if the Unix time in milliseconds cannot be represented by an int64 (a date more than 292 million years before or after 1970). The result does not depend on the location associated with t.

**Returns number**

### unixNano

```ts
unixNano(): number
```

UnixNano returns t as a Unix time, the number of nanoseconds elapsed since January 1, 1970 UTC. The result is undefined if the Unix time in nanoseconds cannot be represented by an int64 (a date before the year 1678 or after 2262). Note that this means the result of calling UnixNano on the zero Time is undefined. The result does not depend on the location associated with t.

**Returns number**

### unmarshalBinary

```ts
unmarshalBinary(data): void
```

UnmarshalBinary implements the [encoding.BinaryUnmarshaler] interface.

**Parameters**

- `data: string | number[]`

**Returns void**

### unmarshalJSON

```ts
unmarshalJSON(b): void
```

UnmarshalJSON is an implementation of the json.RawMessage interface and deserializes a [NumericDate] from a JSON representation, i.e. a [json.Number]. This number represents an UNIX epoch with either integer or non-integer seconds.

**Parameters**

- `b: string | number[]`

**Returns void**

### unmarshalText

```ts
unmarshalText(data): void
```

UnmarshalText implements the [encoding.TextUnmarshaler] interface. The time must be in the RFC 3339 format.

**Parameters**

- `data: string | number[]`

**Returns void**

### utc

```ts
utc(): Time
```

UTC returns t with the location set to UTC.

**Returns Time**

### weekday

```ts
weekday(): Weekday
```

Weekday returns the day of the week specified by t.

**Returns Weekday**

### year

```ts
year(): number
```

Year returns the year in which t occurs.

**Returns number**

### yearDay

```ts
yearDay(): number
```

YearDay returns the day of the year specified by t, in the range [1,365] for non-leap years, and [1,366] in leap years.

**Returns number**

### zone

```ts
zone(): [string, number]
```

Zone computes the time zone in effect at time t, returning the abbreviated name of the zone (such as "CET") and its offset in seconds east of UTC.

**Returns [string, number]**

### zoneBounds

```ts
zoneBounds(): [Time, Time]
```

ZoneBounds returns the bounds of the time zone in effect at time t. The zone begins at start and the next zone begins at end. If the zone begins at the beginning of time, start will be returned as a zero Time. If the zone goes on forever, end will be returned as a zero Time. The Location of the returned times will be the same as t.

**Returns [Time, Time]**
