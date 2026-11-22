Source: https://pocketbase.io/jsvm/interfaces/slog.Level.html

# Interface Level

A Level is the importance or severity of a log event. The higher the level, the more important or severe the event.

**Hierarchy**

- Number

  - Level

## Methods

### appendText

```ts
appendText(b): string | number[]
```

AppendText implements [encoding.TextAppender] by calling [Level.String].

**Parameters**

- `b: string | number[]`

**Returns string | number[]**

### level

```ts
level(): Level
```

Level returns the receiver. It implements [Leveler].

**Returns Level**

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements [encoding/json.Marshaler] by quoting the output of [Level.String].

**Returns string | number[]**

### marshalText

```ts
marshalText(): string | number[]
```

MarshalText implements [encoding.TextMarshaler] by calling [Level.AppendText].

**Returns string | number[]**

### string

```ts
string(): string
```

String returns a name for the level. If the level has a name, then that name in uppercase is returned. If the level is between named values, then an integer is appended to the uppercased name. Examples:

```
 LevelWarn.String() => "WARN"
 (LevelInfo+2).String() => "INFO+2"
```

**Returns string**

### unmarshalJSON

```ts
unmarshalJSON(data): void
```

UnmarshalJSON implements [encoding/json.Unmarshaler] It accepts any string produced by [Level.MarshalJSON], ignoring case. It also accepts numeric offsets that would result in a different string on output. For example, "Error-8" would marshal as "INFO".

**Parameters**

- `data: string | number[]`

**Returns void**

### unmarshalText

```ts
unmarshalText(data): void
```

UnmarshalText implements [encoding.TextUnmarshaler]. It accepts any string produced by [Level.MarshalText], ignoring case. It also accepts numeric offsets that would result in a different string on output. For example, "Error-8" would marshal as "INFO".

**Parameters**

- `data: string | number[]`

**Returns void**
