Source: https://pocketbase.io/jsvm/interfaces/core.MailerEvent.html

# Interface MailerEvent

**Hierarchy**

- [_sfJqVMu](https://pocketbase.io/jsvm/types/core._sfJqVMu.html)

  - MailerEvent

## Methods

### next

```ts
next(): void
```

Next calls the next hook handler.

**Returns void**

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
