Source: https://pocketbase.io/jsvm/interfaces/time.Location.html

# Interface Location

A Location maps time instants to the zone in use at that time. Typically, the Location represents the collection of time offsets in use in a geographical area. For many Locations the time offset varies depending on whether daylight savings time is in use at the time instant.

Location is used to provide a time zone in a printed Time value and for calculations involving intervals that may cross daylight savings time boundaries.

**Hierarchy**

- Location

  - [Timezone](https://pocketbase.io/jsvm/classes/Timezone.html)

**Implemented by**

- [Timezone](https://pocketbase.io/jsvm/classes/Timezone.html)

## Methods

### string

```ts
string(): string
```

String returns a descriptive name for the time zone information, corresponding to the name argument to [LoadLocation] or [FixedZone].

**Returns string**
