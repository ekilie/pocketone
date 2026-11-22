Source: https://pocketbase.io/jsvm/interfaces/types.GeoPoint.html

# Interface GeoPoint

GeoPoint defines a struct for storing geo coordinates as serialized json object (e.g. {lon:0,lat:0}).

Note: using object notation and not a plain array to avoid the confusion as there doesn't seem to be a fixed standard for the coordinates order.

**Hierarchy**

- GeoPoint

## Methods

### asMap

```ts
asMap(): _TygojaDict
```

AsMap implements [core.mapExtractor] and returns a value suitable to be used in an API rule expression.

**Returns _TygojaDict**

### scan

```ts
scan(value): void
```

Scan implements [sql.Scanner] interface to scan the provided value into the current GeoPoint instance.

The value argument could be nil (no-op), another GeoPoint instance, map or serialized json object with lat-lon props.

**Parameters**

- `value: any`

**Returns void**

### string

```ts
string(): string
```

String returns the string representation of the current GeoPoint instance.

**Returns string**

### value

```ts
value(): any
```

Value implements the [driver.Valuer] interface.

**Returns any**

## Properties

### lat

```ts
lat: number
```

### lon

```ts
lon: number
```
