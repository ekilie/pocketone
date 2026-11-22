Source: https://pocketbase.io/jsvm/classes/Timezone.html

# Class Timezone

Timezone returns the timezone location with the given name.

The name is expected to be a location name corresponding to a file in the IANA Time Zone database, such as "America/New_York".

If the name is "Local", LoadLocation returns Local.

If the name is "", invalid or "UTC", returns UTC.

The constructor is equivalent to calling the Go `time.LoadLocation(name)` method.

Example:

```js
const zone = new Timezone("America/New_York")
$app.cron().setTimezone(zone)
```

**Hierarchy**

- [Location](https://pocketbase.io/jsvm/interfaces/time.Location.html)

  - Timezone

**Implements**

- [Location](https://pocketbase.io/jsvm/interfaces/time.Location.html)

## Constructors

### constructor

```ts
new Timezone(name?): Timezone
```

**Parameters**

- `Optional name: string`

**Returns Timezone**

## Methods

### string

```ts
string(): string
```

String returns a descriptive name for the time zone information, corresponding to the name argument to [LoadLocation] or [FixedZone].

**Returns string**
