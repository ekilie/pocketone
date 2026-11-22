Source: https://pocketbase.io/jsvm/interfaces/slog.Record.html

# Interface Record

A Record holds information about a log event. Copies of a Record share state. Do not modify a Record after handing out a copy to it. Call [NewRecord] to create a new Record. Use [Record.Clone] to create a copy with no shared state.

**Hierarchy**

- Record

## Methods

### add

```ts
add(...args): void
```

Add converts the args to Attrs as described in [Logger.Log], then appends the Attrs to the [Record]'s list of Attrs. It omits empty groups.

**Parameters**

- `Rest ...args: any[]`

**Returns void**

### addAttrs

```ts
addAttrs(...attrs): void
```

AddAttrs appends the given Attrs to the [Record]'s list of Attrs. It omits empty groups.

**Parameters**

- `Rest ...attrs: Attr[]`

**Returns void**

### attrs

```ts
attrs(f): void
```

Attrs calls f on each Attr in the [Record]. Iteration stops if f returns false.

**Parameters**

- `f: ((_arg0) => boolean)`

**Returns void**

### clone

```ts
clone(): slog.Record
```

Clone returns a copy of the record with no shared state. The original record and the clone can both be modified without interfering with each other.

**Returns slog.Record**

### numAttrs

```ts
numAttrs(): number
```

NumAttrs returns the number of attributes in the [Record].

**Returns number**

### source

```ts
source(): Source
```

Source returns a new Source for the log event using r's PC. If the PC field is zero, meaning the Record was created without the necessary information or the location is unavailable, then nil is returned.

**Returns Source**

## Properties

### level

```ts
level: Level
```

The level of the event.

### message

```ts
message: string
```

The log message.

### pc

```ts
pc: number
```

The program counter at the time the record was constructed, as determined by runtime.Callers. If zero, no program counter is available.

The only valid use for this value is as an argument to [runtime.CallersFrames]. In particular, it must not be passed to [runtime.FuncForPC].

### time

```ts
time: Time
```

The time at which the output method (Log, Info, etc.) was called.
