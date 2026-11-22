Source: https://pocketbase.io/jsvm/interfaces/mailer.Message.html

# Interface Message

Message defines a generic email message struct.

**Hierarchy**

- Message

  - [MailerMessage](https://pocketbase.io/jsvm/classes/MailerMessage.html)

**Implemented by**

- [MailerMessage](https://pocketbase.io/jsvm/classes/MailerMessage.html)

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
