Source: https://pocketbase.io/jsvm/classes/MailerMessage.html

# Class MailerMessage

MailerMessage defines a single email message.

```js
const message = new MailerMessage({
    from: {
        address: $app.settings().meta.senderAddress,
        name:    $app.settings().meta.senderName,
    },
    to:      [{address: "test@example.com"}],
    subject: "YOUR_SUBJECT...",
    html:    "YOUR_HTML_BODY...",
})

$app.newMailClient().send(message)
```

**Hierarchy**

- [mailer](https://pocketbase.io/jsvm/modules/mailer.html).[Message](https://pocketbase.io/jsvm/interfaces/mailer.Message.html)

  - MailerMessage

**Implements**

- [mailer](https://pocketbase.io/jsvm/modules/mailer.html).[Message](https://pocketbase.io/jsvm/interfaces/mailer.Message.html)

## Constructors

### constructor

```ts
new MailerMessage(message?): MailerMessage
```

**Parameters**

- `Optional message: Partial<mailer.Message>`

**Returns MailerMessage**

## Properties

### attachments

```ts
attachments: _TygojaDict
```

### bcc

```ts
bcc: {     address: string;     name?: string; }[]
```

**Type declaration**

**address: string**

**Optional name?: string**

### cc

```ts
cc: {     address: string;     name?: string; }[]
```

**Type declaration**

**address: string**

**Optional name?: string**

### from

```ts
from: {     address: string;     name?: string; }
```

**Type declaration**

**address: string**

**Optional name?: string**

### headers

```ts
headers: _TygojaDict
```

### html

```ts
html: string
```

### inlineAttachments

```ts
inlineAttachments: _TygojaDict
```

### subject

```ts
subject: string
```

### text

```ts
text: string
```

### to

```ts
to: {     address: string;     name?: string; }[]
```

**Type declaration**

**address: string**

**Optional name?: string**
