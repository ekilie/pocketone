Source: https://pocketbase.io/jsvm/interfaces/slog.Logger.html

# Interface Logger

A Logger records structured information about each call to its Log, Debug, Info, Warn, and Error methods. For each call, it creates a [Record] and passes it to a [Handler].

To create a new Logger, call [New] or a Logger method that begins "With".

**Hierarchy**

- Logger

## Methods

### debug

```ts
debug(msg, ...args): void
```

Debug logs at [LevelDebug]. It uses [context.Background] internally; to specify the context, use [Logger.DebugContext].

**Parameters**

- `msg: string`
- `Rest ...args: any[]`

**Returns void**

### debugContext

```ts
debugContext(ctx, msg, ...args): void
```

DebugContext logs at [LevelDebug] with the given context.

**Parameters**

- `ctx: context.Context`
- `msg: string`
- `Rest ...args: any[]`

**Returns void**

### enabled

```ts
enabled(ctx, level): boolean
```

Enabled reports whether l emits log records at the given context and level.

**Parameters**

- `ctx: context.Context`
- `level: Level`

**Returns boolean**

### error

```ts
error(msg, ...args): void
```

Error logs at [LevelError]. It uses [context.Background] internally; to specify the context, use [Logger.ErrorContext].

**Parameters**

- `msg: string`
- `Rest ...args: any[]`

**Returns void**

### errorContext

```ts
errorContext(ctx, msg, ...args): void
```

ErrorContext logs at [LevelError] with the given context.

**Parameters**

- `ctx: context.Context`
- `msg: string`
- `Rest ...args: any[]`

**Returns void**

### handler

```ts
handler(): slog.Handler
```

Handler returns l's Handler.

**Returns slog.Handler**

### info

```ts
info(msg, ...args): void
```

Info logs at [LevelInfo]. It uses [context.Background] internally; to specify the context, use [Logger.InfoContext].

**Parameters**

- `msg: string`
- `Rest ...args: any[]`

**Returns void**

### infoContext

```ts
infoContext(ctx, msg, ...args): void
```

InfoContext logs at [LevelInfo] with the given context.

**Parameters**

- `ctx: context.Context`
- `msg: string`
- `Rest ...args: any[]`

**Returns void**

### log

```ts
log(ctx, level, msg, ...args): void
```

Log emits a log record with the current time and the given level and message. The Record's Attrs consist of the Logger's attributes followed by the Attrs specified by args.

The attribute arguments are processed as follows:

```
  - If an argument is an Attr, it is used as is.
  - If an argument is a string and this is not the last argument,
    the following argument is treated as the value and the two are combined
    into an Attr.
  - Otherwise, the argument is treated as a value with key "!BADKEY".
```

**Parameters**

- `ctx: context.Context`
- `level: Level`
- `msg: string`
- `Rest ...args: any[]`

**Returns void**

### logAttrs

```ts
logAttrs(ctx, level, msg, ...attrs): void
```

LogAttrs is a more efficient version of [Logger.Log] that accepts only Attrs.

**Parameters**

- `ctx: context.Context`
- `level: Level`
- `msg: string`
- `Rest ...attrs: Attr[]`

**Returns void**

### warn

```ts
warn(msg, ...args): void
```

Warn logs at [LevelWarn]. It uses [context.Background] internally; to specify the context, use [Logger.WarnContext].

**Parameters**

- `msg: string`
- `Rest ...args: any[]`

**Returns void**

### warnContext

```ts
warnContext(ctx, msg, ...args): void
```

WarnContext logs at [LevelWarn] with the given context.

**Parameters**

- `ctx: context.Context`
- `msg: string`
- `Rest ...args: any[]`

**Returns void**

### with

```ts
with(...args): Logger
```

With returns a Logger that includes the given attributes in each output operation. Arguments are converted to attributes as if by [Logger.Log].

**Parameters**

- `Rest ...args: any[]`

**Returns Logger**

### withGroup

```ts
withGroup(name): Logger
```

WithGroup returns a Logger that starts a group, if name is non-empty. The keys of all attributes added to the Logger will be qualified by the given name. (How that qualification happens depends on the [Handler.WithGroup] method of the Logger's Handler.)

If name is empty, WithGroup returns the receiver.

**Parameters**

- `name: string`

**Returns Logger**
