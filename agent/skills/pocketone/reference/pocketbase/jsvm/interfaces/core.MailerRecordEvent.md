Source: https://pocketbase.io/jsvm/interfaces/core.MailerRecordEvent.html

# Interface MailerRecordEvent

**Hierarchy**

- [_shrRGcz](https://pocketbase.io/jsvm/types/core._shrRGcz.html)

  - MailerRecordEvent

## Methods

### next

```ts
next(): void
```

Next calls the next hook handler.

**Returns void**

### tags

```ts
tags(): string[]
```

**Returns string[]**

## Properties

### app

```ts
app: App
```

### extra

```ts
extra: _TygojaDict
```

Extra holds any dynamic data that could be passed between event handlers.

### mailer

```ts
mailer: Mailer
```

### Optional message

```ts
message?: mailer.Message
```

### meta

```ts
meta: _TygojaDict
```

### Optional record

```ts
record?: core.Record
```
