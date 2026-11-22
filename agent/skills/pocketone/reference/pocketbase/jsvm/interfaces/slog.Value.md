Source: https://pocketbase.io/jsvm/interfaces/slog.Value.html

# Interface Value

A Value can represent any Go value, but unlike type any, it can represent most small values without an allocation. The zero Value corresponds to nil.

**Hierarchy**

- Value

## Methods

### any

```ts
any(): any
```

Any returns v's value as an any.

**Returns any**

### bool

```ts
bool(): boolean
```

Bool returns v's value as a bool. It panics if v is not a bool.

**Returns boolean**

### duration

```ts
duration(): Duration
```

Duration returns v's value as a [time.Duration]. It panics if v is not a time.Duration.

**Returns Duration**

### equal

```ts
equal(w): boolean
```

Equal reports whether v and w represent the same Go value.

**Parameters**

- `w: Value`

**Returns boolean**

### float64

```ts
float64(): number
```

Float64 returns v's value as a float64. It panics if v is not a float64.

**Returns number**

### group

```ts
group(): Attr[]
```

Group returns v's value as a []Attr. It panics if v's [Kind] is not [KindGroup].

**Returns Attr[]**

### int64

```ts
int64(): number
```

Int64 returns v's value as an int64. It panics if v is not a signed integer.

**Returns number**

### kind

```ts
kind(): Kind
```

Kind returns v's Kind.

**Returns Kind**

### logValuer

```ts
logValuer(): LogValuer
```

LogValuer returns v's value as a LogValuer. It panics if v is not a LogValuer.

**Returns LogValuer**

### resolve

```ts
resolve(): Value
```

Resolve repeatedly calls LogValue on v while it implements [LogValuer], and returns the result. If v resolves to a group, the group's attributes' values are not recursively resolved. If the number of LogValue calls exceeds a threshold, a Value containing an error is returned. Resolve's return value is guaranteed not to be of Kind [KindLogValuer].

**Returns Value**

### string

```ts
string(): string
```

String returns Value's value as a string, formatted like [fmt.Sprint]. Unlike the methods Int64, Float64, and so on, which panic if v is of the wrong kind, String never panics.

**Returns string**

### time

```ts
time(): Time
```

Time returns v's value as a [time.Time]. It panics if v is not a time.Time.

**Returns Time**

### uint64

```ts
uint64(): number
```

Uint64 returns v's value as a uint64. It panics if v is not an unsigned integer.

**Returns number**
