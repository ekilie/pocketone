Source: https://pocketbase.io/jsvm/interfaces/slog.Handler.html

# Interface Handler

A Handler handles log records produced by a Logger.

A typical handler may print log records to standard error, or write them to a file or database, or perhaps augment them with additional attributes and pass them on to another handler.

Any of the Handler's methods may be called concurrently with itself or with other methods. It is the responsibility of the Handler to manage this concurrency.

Users of the slog package should not invoke Handler methods directly. They should use the methods of [Logger] instead.

Before implementing your own handler, consult [https://go.dev/s/slog-handler-guide](https://go.dev/s/slog-handler-guide).

**Hierarchy**

- Handler

**Indexable**

```ts
[key: string]: any
```

## Methods

### enabled

```ts
enabled(_arg0, _arg1): boolean
```

Enabled reports whether the handler handles records at the given level. The handler ignores records whose level is lower. It is called early, before any arguments are processed, to save effort if the log event should be discarded. If called from a Logger method, the first argument is the context passed to that method, or context.Background() if nil was passed or the method does not take a context. The context is passed so Enabled can use its values to make a decision.

**Parameters**

- `_arg0: context.Context`
- `_arg1: Level`

**Returns boolean**

### handle

```ts
handle(_arg0, _arg1): void
```

Handle handles the Record. It will only be called when Enabled returns true. The Context argument is as for Enabled. It is present solely to provide Handlers access to the context's values. Canceling the context should not affect record processing. (Among other things, log messages may be necessary to debug a cancellation-related problem.)

Handle methods that produce output should observe the following rules:

```
  - If r.Time is the zero time, ignore the time.
  - If r.PC is zero, ignore it.
  - Attr's values should be resolved.
  - If an Attr's key and value are both the zero value, ignore the Attr.
    This can be tested with attr.Equal(Attr{}).
  - If a group's key is empty, inline the group's Attrs.
  - If a group has no Attrs (even if it has a non-empty key),
    ignore it.
```

[Logger] discards any errors from Handle. Wrap the Handle method to process any errors from Handlers.

**Parameters**

- `_arg0: context.Context`
- `_arg1: slog.Record`

**Returns void**

### withAttrs

```ts
withAttrs(attrs): slog.Handler
```

WithAttrs returns a new Handler whose attributes consist of both the receiver's attributes and the arguments. The Handler owns the slice: it may retain, modify or discard it.

**Parameters**

- `attrs: Attr[]`

**Returns slog.Handler**

### withGroup

```ts
withGroup(name): slog.Handler
```

WithGroup returns a new Handler with the given group appended to the receiver's existing groups. The keys of all subsequent attributes, whether added by With or in a Record, should be qualified by the sequence of group names.

How this qualification happens is up to the Handler, so long as this Handler's attribute keys differ from those of another Handler with a different sequence of group names.

A Handler should treat WithGroup as starting a Group of Attrs that ends at the end of the log event. That is,

```
    logger.WithGroup("s").LogAttrs(ctx, level, msg, slog.Int("a", 1), slog.Int("b", 2))
```

should behave like

```
    logger.LogAttrs(ctx, level, msg, slog.Group("s", slog.Int("a", 1), slog.Int("b", 2)))
```

If the name is empty, WithGroup returns the receiver.

**Parameters**

- `name: string`

**Returns slog.Handler**
